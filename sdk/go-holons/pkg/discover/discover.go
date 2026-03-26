// Package discover scans for holon.proto manifests and returns local holon metadata.
package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/organic-programming/go-holons/pkg/identity"
)

// HolonEntry is one discovered holon.
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

// Discover scans a root directory recursively for holon.proto files.
func Discover(root string) ([]HolonEntry, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		trimmed = currentRoot()
	}

	var entries []HolonEntry
	var seen = make(map[string]struct{})
	appendEntries := func(discovered []HolonEntry) {
		for _, entry := range discovered {
			key := entryKey(entry)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			entries = append(entries, entry)
		}
	}

	packages, err := discoverPackagesRecursive(trimmed, "local")
	if err != nil {
		return nil, err
	}
	appendEntries(packages)

	sources, err := discoverSourceInRoot(trimmed, "local")
	if err != nil {
		return nil, err
	}
	appendEntries(sources)

	return entries, nil
}

// DiscoverLocal scans from the current working directory.
func DiscoverLocal() ([]HolonEntry, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return Discover(root)
}

// DiscoverAll scans the local root, then $OPBIN, then $OPPATH/cache.
func DiscoverAll() ([]HolonEntry, error) {
	seen := make(map[string]struct{})
	entries := make([]HolonEntry, 0)
	appendEntries := func(discovered []HolonEntry) {
		for _, entry := range discovered {
			key := entryKey(entry)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			entries = append(entries, entry)
		}
	}

	roots := []struct {
		discover func() ([]HolonEntry, error)
	}{
		{discover: func() ([]HolonEntry, error) {
			root := bundleHolonsRoot()
			if root == "" {
				return nil, nil
			}
			return discoverPackagesDirect(root, "bundle")
		}},
		{discover: func() ([]HolonEntry, error) {
			return discoverPackagesDirect(buildPackagesRoot(), "build")
		}},
		{discover: func() ([]HolonEntry, error) {
			return discoverPackagesDirect(opbin(), "$OPBIN")
		}},
		{discover: func() ([]HolonEntry, error) {
			return discoverPackagesRecursive(cacheDir(), "cache")
		}},
		{discover: func() ([]HolonEntry, error) {
			return discoverPackagesRecursive(currentRoot(), "local")
		}},
		{discover: func() ([]HolonEntry, error) {
			return discoverSourceInRoot(currentRoot(), "local")
		}},
	}

	for _, root := range roots {
		discovered, err := root.discover()
		if err != nil {
			return nil, err
		}
		appendEntries(discovered)
	}

	return entries, nil
}

// FindBySlug resolves a holon by slug across the standard discover roots.
func FindBySlug(slug string) (*HolonEntry, error) {
	needle := strings.TrimSpace(slug)
	if needle == "" {
		return nil, nil
	}

	entries, err := DiscoverAll()
	if err != nil {
		return nil, err
	}

	var match *HolonEntry
	for i := range entries {
		if entries[i].Slug != needle {
			continue
		}
		if match != nil && match.UUID != entries[i].UUID {
			return nil, fmt.Errorf("ambiguous holon %q", needle)
		}
		entry := entries[i]
		match = &entry
	}

	return match, nil
}

// FindByUUID resolves a holon by UUID prefix across the standard discover roots.
func FindByUUID(prefix string) (*HolonEntry, error) {
	needle := strings.TrimSpace(prefix)
	if needle == "" {
		return nil, nil
	}

	entries, err := DiscoverAll()
	if err != nil {
		return nil, err
	}

	var match *HolonEntry
	for i := range entries {
		if !strings.HasPrefix(entries[i].UUID, needle) {
			continue
		}
		if match != nil && match.UUID != entries[i].UUID {
			return nil, fmt.Errorf("ambiguous UUID prefix %q", needle)
		}
		entry := entries[i]
		match = &entry
	}

	return match, nil
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
			Entrypoint:   resolved.ArtifactBinary,
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
		entry, ok := entriesByKey[key]
		if ok {
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

func buildPackagesRoot() string {
	return filepath.Join(currentRoot(), ".op", "build")
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
