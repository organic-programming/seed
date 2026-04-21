// Package discover scans for holon.proto manifests and returns local holon metadata.
package discover

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/organic-programming/go-holons/pkg/identity"
)

// HolonEntry is the internal discover representation used by the Go SDK.
type HolonEntry struct {
	Slug          string
	UUID          string
	Dir           string
	RelativePath  string
	Origin        string
	Identity      identity.Identity
	Manifest      *Manifest
	SourceKind    string
	PackageRoot   string
	Runner        string
	Transport     string
	Entrypoint    string
	Architectures []string
	HasDist       bool
	HasSource     bool
}

// Manifest contains the operational fields needed by discover/connect.
type Manifest struct {
	Kind      string
	Build     Build
	Artifacts Artifacts
}

// Build contains the build runner metadata from holon.proto.
type Build struct {
	Runner string
	Main   string
}

// Artifacts contains the primary artifact names from holon.proto.
type Artifacts struct {
	Binary  string
	Primary string
}

type discoverLayer struct {
	flag int
	name string
	scan func(root string) ([]HolonEntry, error)
}

// Discover scans for holons matching the given criteria.
func Discover(scope int, expression *string, root *string, specifiers int, limit int, timeout int) DiscoverResult {
	if scope != LOCAL {
		return DiscoverResult{Error: fmt.Sprintf("scope %d not supported", scope)}
	}
	if specifiers < 0 || specifiers > ALL {
		return DiscoverResult{Error: fmt.Sprintf("invalid specifiers 0x%02X: valid range is 0x00-0x3F", specifiers)}
	}
	if specifiers == 0 {
		specifiers = ALL
	}
	if limit < 0 {
		return DiscoverResult{Found: []HolonRef{}}
	}

	ctx, cancel := discoverContext(timeout)
	defer cancel()

	expr := normalizedExpression(expression)
	if expr != nil && isDirectTransportExpression(*expr) {
		return DiscoverResult{Found: applyRefLimit([]HolonRef{discoverTransportRef(ctx, *expr)}, limit)}
	}

	var searchRoot string
	var err error
	resolveRoot := func() (string, error) {
		if searchRoot != "" {
			return searchRoot, nil
		}
		resolvedRoot, err := resolveDiscoverRoot(root)
		if err != nil {
			return "", err
		}
		searchRoot = resolvedRoot
		return searchRoot, nil
	}

	if expr != nil {
		refs, handled, pathErr := discoverPathExpression(ctx, *expr, resolveRoot)
		if pathErr != nil {
			return DiscoverResult{Error: pathErr.Error()}
		}
		if handled {
			return DiscoverResult{Found: applyRefLimit(refs, limit)}
		}
	}

	searchRoot, err = resolveRoot()
	if err != nil {
		return DiscoverResult{Error: err.Error()}
	}

	entries, err := discoverEntries(ctx, searchRoot, specifiers)
	if err != nil {
		return DiscoverResult{Error: err.Error()}
	}

	found := make([]HolonRef, 0, len(entries))
	for _, entry := range entries {
		if err := contextError(ctx); err != nil {
			return DiscoverResult{Error: err.Error()}
		}
		if !matchesExpression(entry, expr) {
			continue
		}
		found = append(found, holonRefFromEntry(entry))
		if limit > 0 && len(found) >= limit {
			break
		}
	}

	return DiscoverResult{Found: found}
}

func Resolve(scope int, expression string, root *string, specifiers int, timeout int) ResolveResult {
	expr := expression
	result := Discover(scope, &expr, root, specifiers, 1, timeout)
	if result.Error != "" {
		return ResolveResult{Error: result.Error}
	}
	if len(result.Found) == 0 {
		return ResolveResult{Error: fmt.Sprintf("holon %q not found", expression)}
	}
	if result.Found[0].Error != "" {
		ref := result.Found[0]
		return ResolveResult{Ref: &ref, Error: ref.Error}
	}
	return ResolveResult{Ref: &result.Found[0]}
}

func discoverEntries(ctx context.Context, root string, specifiers int) ([]HolonEntry, error) {
	layers := []discoverLayer{
		{
			flag: SIBLINGS,
			name: "siblings",
			scan: func(string) ([]HolonEntry, error) {
				bundleRoot := bundleHolonsRoot()
				if bundleRoot == "" {
					return nil, nil
				}
				return discoverPackagesDirect(bundleRoot, "siblings")
			},
		},
		{
			flag: CWD,
			name: "cwd",
			scan: func(root string) ([]HolonEntry, error) {
				return discoverPackagesRecursive(root, "cwd")
			},
		},
		{
			flag: SOURCE,
			name: "source",
			scan: func(root string) ([]HolonEntry, error) {
				return discoverSourceInRoot(root, "source")
			},
		},
		{
			flag: BUILT,
			name: "built",
			scan: func(root string) ([]HolonEntry, error) {
				return discoverPackagesDirect(filepath.Join(root, ".op", "build"), "built")
			},
		},
		{
			flag: INSTALLED,
			name: "installed",
			scan: func(string) ([]HolonEntry, error) {
				return discoverPackagesDirect(opbin(), "installed")
			},
		},
		{
			flag: CACHED,
			name: "cached",
			scan: func(string) ([]HolonEntry, error) {
				return discoverPackagesRecursive(cacheDir(), "cached")
			},
		},
	}

	seen := make(map[string]struct{})
	found := make([]HolonEntry, 0)

	for _, layer := range layers {
		if specifiers&layer.flag == 0 {
			continue
		}
		if err := contextError(ctx); err != nil {
			return nil, err
		}

		entries, err := layer.scan(root)
		if err != nil {
			return nil, fmt.Errorf("scan %s layer: %w", layer.name, err)
		}

		for _, entry := range entries {
			if err := contextError(ctx); err != nil {
				return nil, err
			}
			key := entryKey(entry)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			found = append(found, entry)
		}
	}

	return found, nil
}

func discoverSourceInRoot(root, origin string) ([]HolonEntry, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		trimmed = currentRoot()
	}
	absRoot, err := filepath.Abs(trimmed)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	entriesByKey := make(map[string]HolonEntry)
	keys := make([]string, 0)
	protoFiles := make([]string, 0)

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(absRoot, path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == identity.ProtoManifestFileName {
			protoFiles = append(protoFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(protoFiles)
	for _, protoPath := range protoFiles {
		resolved, resolveErr := identity.ResolveProtoFile(protoPath)
		if resolveErr != nil {
			continue
		}
		if resolved.Identity.GivenName == "" && resolved.Identity.FamilyName == "" {
			continue
		}

		dir := protoHolonDir(absRoot, protoPath, resolved)
		absDir, _ := filepath.Abs(dir)
		entry := HolonEntry{
			Slug:         resolved.Identity.Slug(),
			UUID:         resolved.Identity.UUID,
			Dir:          absDir,
			RelativePath: relativePath(absRoot, absDir),
			Origin:       origin,
			Identity:     resolved.Identity,
			Manifest:     manifestFromResolved(resolved),
			SourceKind:   "source",
			Runner:       resolved.BuildRunner,
			Transport:    resolved.Transport,
			Entrypoint:   resolved.ArtifactBinary,
			HasSource:    true,
		}

		key := strings.TrimSpace(entry.UUID)
		if key == "" {
			key = entry.Dir
		}
		if existing, ok := entriesByKey[key]; ok {
			if shouldReplaceEntry(existing, entry) {
				entriesByKey[key] = entry
			}
			continue
		}
		entriesByKey[key] = entry
		keys = append(keys, key)
	}

	entries := make([]HolonEntry, 0, len(entriesByKey))
	for _, key := range keys {
		if entry, ok := entriesByKey[key]; ok {
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RelativePath == entries[j].RelativePath {
			return entries[i].UUID < entries[j].UUID
		}
		return entries[i].RelativePath < entries[j].RelativePath
	})

	return entries, nil
}

func entryKey(entry HolonEntry) string {
	key := strings.TrimSpace(entry.UUID)
	if key != "" {
		return key
	}
	if entry.PackageRoot != "" {
		return filepath.Clean(entry.PackageRoot)
	}
	return filepath.Clean(entry.Dir)
}

func manifestFromResolved(resolved *identity.ResolvedManifest) *Manifest {
	if resolved == nil {
		return nil
	}
	return &Manifest{
		Kind: resolved.Kind,
		Build: Build{
			Runner: resolved.BuildRunner,
			Main:   resolved.BuildMain,
		},
		Artifacts: Artifacts{
			Binary:  resolved.ArtifactBinary,
			Primary: resolved.ArtifactPrimary,
		},
	}
}

var protoVersionDirPattern = regexp.MustCompile(`^v[0-9]+(?:[A-Za-z0-9._-]*)?$`)

func shouldReplaceEntry(current, next HolonEntry) bool {
	currentDepth := pathDepth(current.RelativePath)
	nextDepth := pathDepth(next.RelativePath)
	return nextDepth < currentDepth
}

func protoHolonDir(root, protoPath string, resolved *identity.ResolvedManifest) string {
	protoDir := filepath.Dir(protoPath)
	bestDir := protoDir
	bestScore := 0
	bestDepth := pathDepth(relativePath(root, protoDir))

	for candidate := protoDir; candidate != ""; candidate = filepath.Dir(candidate) {
		if !isWithinRoot(root, candidate) {
			break
		}

		score := protoCandidateScore(candidate, resolved)
		depth := pathDepth(relativePath(root, candidate))
		if score > bestScore || (score == bestScore && score > 0 && depth > bestDepth) {
			bestDir = candidate
			bestScore = score
			bestDepth = depth
		}

		if filepath.Clean(candidate) == filepath.Clean(root) {
			break
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
	}

	if bestScore > 0 {
		return bestDir
	}

	if protoVersionDirPattern.MatchString(filepath.Base(protoDir)) {
		parent := filepath.Dir(protoDir)
		if parent != protoDir && isWithinRoot(root, parent) {
			return parent
		}
	}

	return protoDir
}

func protoCandidateScore(dir string, resolved *identity.ResolvedManifest) int {
	if resolved == nil {
		return 0
	}

	score := 0
	for _, requiredFile := range resolved.RequiredFiles {
		if protoCandidatePathExists(dir, requiredFile) {
			score++
		}
	}
	if protoCandidatePathExists(dir, resolved.BuildMain) {
		score++
	}
	for _, memberPath := range resolved.MemberPaths {
		if protoCandidatePathExists(dir, memberPath) {
			score++
		}
	}
	if protoCandidatePathExists(dir, resolved.ArtifactPrimary) {
		score++
	}
	return score
}

func protoCandidatePathExists(base, rel string) bool {
	path, ok := resolveProtoCandidatePath(base, rel)
	if !ok {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func resolveProtoCandidatePath(base, rel string) (string, bool) {
	trimmed := strings.TrimSpace(rel)
	if trimmed == "" {
		return "", false
	}

	candidate := filepath.Clean(filepath.Join(base, filepath.FromSlash(trimmed)))
	withinBase, err := filepath.Rel(base, candidate)
	if err != nil {
		return "", false
	}
	if withinBase == ".." || strings.HasPrefix(withinBase, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return candidate, true
}

func isWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func shouldSkipDir(root, path, name string) bool {
	if filepath.Clean(path) == filepath.Clean(root) {
		return false
	}
	if strings.HasSuffix(name, ".holon") {
		return false
	}
	if name == ".git" || name == ".op" || name == "node_modules" || name == "vendor" || name == "build" || name == "testdata" {
		return true
	}
	return strings.HasPrefix(name, ".")
}

func relativePath(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(rel)
}

func pathDepth(rel string) int {
	trimmed := strings.Trim(strings.TrimSpace(filepath.ToSlash(rel)), "/")
	if trimmed == "" || trimmed == "." {
		return 0
	}
	return len(strings.Split(trimmed, "/"))
}

func currentRoot() string {
	cwd, err := os.Getwd()
	if err != nil || strings.TrimSpace(cwd) == "" {
		return "."
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		return abs
	}
	return cwd
}

var executablePath = os.Executable

func bundleHolonsRoot() string {
	executable, err := executablePath()
	if err != nil || strings.TrimSpace(executable) == "" {
		return ""
	}

	current := filepath.Dir(executable)
	for {
		if strings.HasSuffix(strings.ToLower(current), ".app") {
			candidate := filepath.Join(current, "Contents", "Resources", "Holons")
			info, statErr := os.Stat(candidate)
			if statErr == nil && info.IsDir() {
				return candidate
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

func oppath() string {
	if root := strings.TrimSpace(os.Getenv("OPPATH")); root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			return abs
		}
		return filepath.Clean(root)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ".op"
	}
	return filepath.Join(home, ".op")
}

func opbin() string {
	if root := strings.TrimSpace(os.Getenv("OPBIN")); root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			return abs
		}
		return filepath.Clean(root)
	}
	return filepath.Join(oppath(), "bin")
}

func cacheDir() string {
	return filepath.Join(oppath(), "cache")
}

func resolveDiscoverRoot(root *string) (string, error) {
	if root == nil {
		return currentRoot(), nil
	}

	trimmed := strings.TrimSpace(*root)
	if trimmed == "" {
		return "", fmt.Errorf("root cannot be empty")
	}

	absRoot, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root %q is not a directory", trimmed)
	}
	return absRoot, nil
}

func discoverContext(timeout int) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		if err == context.DeadlineExceeded {
			return fmt.Errorf("discover timed out")
		}
		return err
	}
	return nil
}

func normalizedExpression(expression *string) *string {
	if expression == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*expression)
	return &trimmed
}

func discoverPathExpression(ctx context.Context, expression string, rootResolver func() (string, error)) ([]HolonRef, bool, error) {
	candidate, ok, err := pathExpressionCandidate(expression, rootResolver)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	if err := contextError(ctx); err != nil {
		return nil, true, err
	}

	ref, found, err := discoverRefAtPath(candidate)
	if err != nil {
		return nil, true, err
	}
	if !found {
		return []HolonRef{}, true, nil
	}
	return []HolonRef{ref}, true, nil
}

func pathExpressionCandidate(expression string, rootResolver func() (string, error)) (string, bool, error) {
	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return "", false, nil
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		path, err := pathFromFileURL(trimmed)
		if err != nil {
			return "", false, err
		}
		return path, true, nil
	}
	if !(filepath.IsAbs(trimmed) ||
		strings.HasPrefix(trimmed, ".") ||
		strings.Contains(trimmed, string(os.PathSeparator)) ||
		strings.Contains(trimmed, "/") ||
		strings.Contains(trimmed, "\\") ||
		strings.HasSuffix(strings.ToLower(trimmed), ".holon")) {
		return "", false, nil
	}
	if filepath.IsAbs(trimmed) {
		return trimmed, true, nil
	}
	root, err := rootResolver()
	if err != nil {
		return "", false, err
	}
	return filepath.Join(root, trimmed), true, nil
}

func discoverRefAtPath(path string) (HolonRef, bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return HolonRef{}, false, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return HolonRef{}, false, nil
		}
		return HolonRef{}, false, err
	}

	if info.IsDir() {
		if strings.HasSuffix(info.Name(), ".holon") || hasHolonJSON(absPath) {
			root := filepath.Dir(absPath)
			entry, loadErr := loadPackageEntry(root, absPath, "path")
			if loadErr == nil {
				return holonRefFromEntry(entry), true, nil
			}
			entry, probeErr := probePackageEntry(root, absPath, "path")
			if probeErr == nil {
				return holonRefFromEntry(entry), true, nil
			}
			return HolonRef{URL: fileURL(absPath), Error: probeErr.Error()}, true, nil
		}

		entries, discoverErr := discoverSourceInRoot(absPath, "path")
		if discoverErr != nil {
			return HolonRef{}, false, discoverErr
		}
		if len(entries) == 1 {
			return holonRefFromEntry(entries[0]), true, nil
		}
		for _, entry := range entries {
			if filepath.Clean(entry.Dir) == filepath.Clean(absPath) {
				return holonRefFromEntry(entry), true, nil
			}
		}
		return HolonRef{}, false, nil
	}

	if filepath.Base(absPath) == identity.ProtoManifestFileName {
		entries, discoverErr := discoverSourceInRoot(filepath.Dir(absPath), "path")
		if discoverErr != nil {
			return HolonRef{}, false, discoverErr
		}
		if len(entries) == 1 {
			return holonRefFromEntry(entries[0]), true, nil
		}
		return HolonRef{}, false, nil
	}

	entry, probeErr := probeBinaryPath(absPath)
	if probeErr == nil {
		return holonRefFromEntry(entry), true, nil
	}
	return HolonRef{URL: fileURL(absPath), Error: probeErr.Error()}, true, nil
}

func hasHolonJSON(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".holon.json"))
	return err == nil && !info.IsDir()
}

func matchesExpression(entry HolonEntry, expression *string) bool {
	if expression == nil {
		return true
	}

	needle := strings.TrimSpace(*expression)
	if needle == "" {
		return false
	}
	if entry.Slug == needle {
		return true
	}
	if strings.HasPrefix(entry.UUID, needle) {
		return true
	}
	for _, alias := range entry.Identity.Aliases {
		if alias == needle {
			return true
		}
	}

	base := strings.TrimSuffix(filepath.Base(entry.Dir), ".holon")
	return base == needle
}

func holonRefFromEntry(entry HolonEntry) HolonRef {
	return HolonRef{
		URL:  fileURL(entry.Dir),
		Info: holonInfoFromEntry(entry),
	}
}

func holonInfoFromEntry(entry HolonEntry) *HolonInfo {
	runner := entry.Runner
	kind := ""
	buildMain := ""
	if entry.Manifest != nil {
		if runner == "" {
			runner = entry.Manifest.Build.Runner
		}
		kind = entry.Manifest.Kind
		buildMain = entry.Manifest.Build.Main
	}

	return &HolonInfo{
		Slug: entry.Slug,
		UUID: entry.UUID,
		Identity: IdentityInfo{
			GivenName:  entry.Identity.GivenName,
			FamilyName: entry.Identity.FamilyName,
			Motto:      entry.Identity.Motto,
			Aliases:    append([]string(nil), entry.Identity.Aliases...),
		},
		Lang:          entry.Identity.Lang,
		Runner:        runner,
		Status:        entry.Identity.Status,
		Kind:          kind,
		Transport:     entry.Transport,
		Entrypoint:    entry.Entrypoint,
		Architectures: append([]string(nil), entry.Architectures...),
		HasDist:       entry.HasDist,
		HasSource:     entry.HasSource,
		BuildMain:     buildMain,
		SourceKind:    entry.SourceKind,
	}
}

func fileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func applyRefLimit(refs []HolonRef, limit int) []HolonRef {
	if limit <= 0 || len(refs) <= limit {
		return refs
	}
	return refs[:limit]
}
