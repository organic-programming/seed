package holons

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/identity"
)

const localLayerSpecifiers = sdkdiscover.ALL &^ sdkdiscover.CACHED

var protoVersionDirPattern = regexp.MustCompile(`^v[0-9]+(?:[A-Za-z0-9._-]*)?$`)

type Target struct {
	Ref          string
	Dir          string
	RelativePath string
	Identity     *identity.Identity
	IdentityPath string
	Manifest     *LoadedManifest
	ManifestErr  error
}

type LocalHolon struct {
	Dir          string
	RelativePath string
	Origin       string
	Identity     identity.Identity
	IdentityPath string
	Manifest     *LoadedManifest
}

func KnownRoots() []string {
	return []string{openv.Root()}
}

func KnownRootLabels() []string {
	return []string{openv.Root()}
}

func DiscoverRefs(expression *string, root *string, specifiers int, limit int, timeout int) sdkdiscover.DiscoverResult {
	resolvedRoot := effectiveDiscoverRoot(root)
	return sdkdiscover.Discover(sdkdiscover.LOCAL, expression, resolvedRoot, specifiers, limit, timeout)
}

func ResolveRef(expression string, root *string, specifiers int, timeout int) sdkdiscover.ResolveResult {
	resolvedRoot := effectiveDiscoverRoot(root)
	return sdkdiscover.Resolve(sdkdiscover.LOCAL, expression, resolvedRoot, specifiers, timeout)
}

func ConnectRef(expression string, root *string, specifiers int, timeout int) sdkconnect.ConnectResult {
	resolvedRoot := effectiveDiscoverRoot(root)
	return sdkconnect.Connect(sdkdiscover.LOCAL, expression, resolvedRoot, specifiers, timeout)
}

func DiscoverHolonsWithOptions(root *string, specifiers int, limit int, timeout int) ([]LocalHolon, error) {
	result := DiscoverRefs(nil, root, specifiers, limit, timeout)
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	resolvedRoot := openv.Root()
	if effective := effectiveDiscoverRoot(root); effective != nil {
		resolvedRoot = strings.TrimSpace(*effective)
	}
	return localHolonsFromRefs(result.Found, resolvedRoot)
}

func DiscoverHolons(root string) ([]LocalHolon, error) {
	return DiscoverHolonsWithOptions(&root, sdkdiscover.ALL, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
}

func DiscoverLocalHolons() ([]LocalHolon, error) {
	root := openv.Root()
	return DiscoverHolonsWithOptions(&root, localLayerSpecifiers, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
}

func DiscoverCachedHolons() ([]LocalHolon, error) {
	root := openv.CacheDir()
	return DiscoverHolonsWithOptions(&root, sdkdiscover.CACHED, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
}

func ResolveTarget(ref string) (*Target, error) {
	return ResolveTargetWithOptions(ref, nil, sdkdiscover.ALL, sdkdiscover.NO_TIMEOUT)
}

func ResolveTargetWithOptions(ref string, root *string, specifiers int, timeout int) (*Target, error) {
	result := ResolveRef(ref, root, specifiers, timeout)
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	target, err := targetFromRef(result.Ref)
	if err != nil {
		return nil, err
	}
	if trimmed := strings.TrimSpace(ref); trimmed != "" {
		target.Ref = trimmed
	}
	return target, nil
}

func OriginDetails(ref *sdkdiscover.HolonRef, root *string) (string, string, error) {
	if ref == nil {
		return "", "", fmt.Errorf("no holon ref")
	}
	path, err := pathFromRefURL(ref.URL)
	if err != nil {
		return "", "", err
	}
	resolvedRoot := openv.Root()
	if effective := effectiveDiscoverRoot(root); effective != nil {
		resolvedRoot = strings.TrimSpace(*effective)
	}
	layer := inferOrigin(*ref, path, resolvedRoot)
	return path, layer, nil
}

func ResolveBinary(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("holon %q not found", name)
	}

	if target, err := ResolveTarget(trimmed); err == nil {
		if binaryPath := builtBinaryForTarget(target); binaryPath != "" {
			return binaryPath, nil
		}
		if systemPath := lookupBinaryOnSystem(binaryLookupNames(target, trimmed)...); systemPath != "" {
			return systemPath, nil
		}
	} else if !isTargetNotFound(err) {
		return "", err
	}

	if systemPath := lookupBinaryOnSystem(trimmed); systemPath != "" {
		return systemPath, nil
	}

	return "", fmt.Errorf("holon %q not found", name)
}

func ResolveInstalledBinary(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return lookupBinaryOnSystem(trimmed)
}

func targetFromRef(ref *sdkdiscover.HolonRef) (*Target, error) {
	if ref == nil {
		return nil, fmt.Errorf("no holon ref")
	}

	refPath, err := pathFromRefURL(ref.URL)
	if err != nil {
		return nil, err
	}
	sourceKind := ""
	if ref.Info != nil {
		sourceKind = strings.TrimSpace(ref.Info.SourceKind)
	}

	dirPath := refPath
	if sourceKind == "binary" {
		dirPath = filepath.Dir(refPath)
	}
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, err
	}

	target := &Target{
		Ref:          referenceLabel(ref, refPath),
		Dir:          absDir,
		RelativePath: workspaceRelativePath(absDir),
	}

	if ref.Info != nil {
		id := identityFromInfo(ref.Info)
		target.Identity = &id
		target.IdentityPath = inferredIdentityPath(absDir, sourceKind)
	}

	manifest, loadErr := LoadManifest(absDir)
	if loadErr == nil {
		target.Manifest = manifest
		if target.IdentityPath == "" {
			target.IdentityPath = manifest.Path
		}
		if target.Identity == nil {
			id := identityFromLoadedManifest(manifest)
			target.Identity = &id
		}
		return target, nil
	}

	if target.IdentityPath != "" || !errors.Is(loadErr, errProtoManifestNotFound) {
		target.ManifestErr = loadErr
	}
	return target, nil
}

func localHolonsFromRefs(refs []sdkdiscover.HolonRef, root string) ([]LocalHolon, error) {
	located := make([]LocalHolon, 0, len(refs))
	for _, ref := range refs {
		if ref.Error != "" {
			continue
		}
		entry, err := localHolonFromRef(ref, root)
		if err != nil {
			continue
		}
		located = append(located, entry)
	}

	sort.Slice(located, func(i, j int) bool {
		if located[i].Origin == located[j].Origin {
			if located[i].RelativePath == located[j].RelativePath {
				return located[i].Identity.UUID < located[j].Identity.UUID
			}
			return located[i].RelativePath < located[j].RelativePath
		}
		return located[i].Origin < located[j].Origin
	})
	return located, nil
}

func localHolonFromRef(ref sdkdiscover.HolonRef, root string) (LocalHolon, error) {
	target, err := targetFromRef(&ref)
	if err != nil {
		return LocalHolon{}, err
	}

	id := identity.Identity{}
	if target.Identity != nil {
		id = *target.Identity
	}

	origin := inferOrigin(ref, target.Dir, root)
	return LocalHolon{
		Dir:          target.Dir,
		RelativePath: relativePathForOrigin(origin, target.Dir, root),
		Origin:       origin,
		Identity:     id,
		IdentityPath: target.IdentityPath,
		Manifest:     target.Manifest,
	}, nil
}

func inferOrigin(ref sdkdiscover.HolonRef, dir string, root string) string {
	cleanDir := filepath.Clean(dir)
	cleanRoot := filepath.Clean(strings.TrimSpace(root))

	switch {
	case isWithinBase(filepath.Join(cleanRoot, ".op", "build"), cleanDir):
		return "built"
	case isWithinBase(openv.OPBIN(), cleanDir):
		return "installed"
	case isWithinBase(openv.CacheDir(), cleanDir):
		return "cached"
	case isWithinSiblingBundle(cleanDir):
		return "siblings"
	case ref.Info != nil && strings.TrimSpace(ref.Info.SourceKind) == "source" && isWithinBase(cleanRoot, cleanDir):
		return "source"
	case isWithinBase(cleanRoot, cleanDir):
		return "cwd"
	default:
		return "path"
	}
}

func relativePathForOrigin(origin string, dir string, root string) string {
	base := strings.TrimSpace(root)
	switch origin {
	case "built":
		base = filepath.Join(base, ".op", "build")
	case "installed":
		base = openv.OPBIN()
	case "cached":
		base = openv.CacheDir()
	case "siblings":
		base = siblingBundleRoot(dir)
	case "path":
		return filepath.ToSlash(dir)
	}
	if base == "" {
		return filepath.ToSlash(dir)
	}
	if rel, err := filepath.Rel(base, dir); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(dir)
}

func isWithinSiblingBundle(path string) bool {
	return siblingBundleRoot(path) != ""
}

func siblingBundleRoot(path string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for i := 0; i+3 < len(parts); i++ {
		if parts[i] == "Contents" && parts[i+1] == "Resources" && parts[i+2] == "Holons" {
			return filepath.FromSlash(strings.Join(parts[:i+3], "/"))
		}
	}
	return ""
}

func isWithinBase(base string, candidate string) bool {
	trimmedBase := strings.TrimSpace(base)
	if trimmedBase == "" {
		return false
	}
	rel, err := filepath.Rel(trimmedBase, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func effectiveDiscoverRoot(root *string) *string {
	if root != nil {
		return root
	}
	resolved := openv.Root()
	return &resolved
}

func identityFromInfo(info *sdkdiscover.HolonInfo) identity.Identity {
	if info == nil {
		return identity.Identity{}
	}
	return identity.Identity{
		UUID:       strings.TrimSpace(info.UUID),
		GivenName:  strings.TrimSpace(info.Identity.GivenName),
		FamilyName: strings.TrimSpace(info.Identity.FamilyName),
		Motto:      strings.TrimSpace(info.Identity.Motto),
		Status:     strings.TrimSpace(info.Status),
		Lang:       strings.TrimSpace(info.Lang),
		Aliases:    append([]string(nil), info.Identity.Aliases...),
	}
}

func referenceLabel(ref *sdkdiscover.HolonRef, refPath string) string {
	if ref != nil && ref.Info != nil && strings.TrimSpace(ref.Info.Slug) != "" {
		return strings.TrimSpace(ref.Info.Slug)
	}
	if trimmed := strings.TrimSpace(refPath); trimmed != "" {
		return trimmed
	}
	if ref != nil {
		return strings.TrimSpace(ref.URL)
	}
	return ""
}

func inferredIdentityPath(dir string, sourceKind string) string {
	switch {
	case sourceKind == "package" || isHolonPackagePath(dir):
		path := filepath.Join(dir, ".holon.json")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func shouldSkipDiscoveryDir(root, path, name string) bool {
	if filepath.Clean(path) == filepath.Clean(root) {
		return false
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), ".holon") {
		return false
	}
	if name == ".git" || name == ".op" || name == "node_modules" || name == "vendor" || name == "build" || name == "testdata" {
		return true
	}
	return strings.HasPrefix(name, ".")
}

func protoHolonDir(root, protoPath string, resolved *identity.Resolved) string {
	protoDir := filepath.Dir(protoPath)
	bestDir := protoDir
	bestScore := 0
	bestDepth := discoveryPathDepth(holonRelativePath(root, protoDir))

	for candidate := protoDir; candidate != ""; candidate = filepath.Dir(candidate) {
		if !isWithinDiscoveryRoot(root, candidate) {
			break
		}

		score := protoCandidateScore(candidate, resolved)
		depth := discoveryPathDepth(holonRelativePath(root, candidate))
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
		if layoutDir := filepath.Base(parent); layoutDir == "api" || layoutDir == "protos" {
			grandparent := filepath.Dir(parent)
			if grandparent != parent && isWithinDiscoveryRoot(root, grandparent) {
				return grandparent
			}
		}
		if parent != protoDir && isWithinDiscoveryRoot(root, parent) {
			return parent
		}
	}

	return protoDir
}

func protoCandidateScore(dir string, resolved *identity.Resolved) int {
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
	if protoCandidatePathExists(dir, resolved.PrimaryArtifact) {
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

func isWithinDiscoveryRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func discoveryPathDepth(rel string) int {
	trimmed := strings.Trim(strings.TrimSpace(filepath.ToSlash(rel)), "/")
	if trimmed == "" || trimmed == "." {
		return 0
	}
	return len(strings.Split(trimmed, "/"))
}

func identityFromLoadedManifest(manifest *LoadedManifest) identity.Identity {
	if manifest == nil {
		return identity.Identity{}
	}

	return identity.Identity{
		Schema:       manifest.Manifest.Schema,
		UUID:         manifest.Manifest.UUID,
		GivenName:    manifest.Manifest.GivenName,
		FamilyName:   manifest.Manifest.FamilyName,
		Motto:        manifest.Manifest.Motto,
		Composer:     manifest.Manifest.Composer,
		Clade:        manifest.Manifest.Clade,
		Status:       manifest.Manifest.Status,
		Born:         manifest.Manifest.Born,
		Version:      manifest.Manifest.Version,
		Parents:      append([]string(nil), manifest.Manifest.Parents...),
		Reproduction: manifest.Manifest.Reproduction,
		Aliases:      append([]string(nil), manifest.Manifest.Aliases...),
		GeneratedBy:  manifest.Manifest.GeneratedBy,
		Lang:         manifest.Manifest.Lang,
		ProtoStatus:  manifest.Manifest.ProtoStatus,
		Description:  manifest.Manifest.Description,
	}
}

func readHolonPackageJSON(pkgDir string) (*HolonPackageJSON, error) {
	data, err := os.ReadFile(filepath.Join(pkgDir, ".holon.json"))
	if err != nil {
		return nil, err
	}
	var pkg HolonPackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func builtBinaryForTarget(target *Target) string {
	if target == nil {
		return ""
	}
	if target.Manifest != nil {
		binaryPath := target.Manifest.BinaryPath()
		if binaryPath != "" {
			if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
				return binaryPath
			}
		}
		if binaryName := target.Manifest.BinaryName(); binaryName != "" {
			legacyPath := filepath.Join(target.Dir, ".op", "build", "bin", binaryName)
			if info, err := os.Stat(legacyPath); err == nil && !info.IsDir() {
				return legacyPath
			}
		}
	}
	if isHolonPackagePath(target.Dir) {
		if pkg, err := readHolonPackageJSON(target.Dir); err == nil && pkg.Entrypoint != "" {
			if bp := PackageBinaryPath(target.Dir, pkg.Entrypoint); bp != "" {
				if info, err := os.Stat(bp); err == nil && !info.IsDir() {
					return bp
				}
			}
		}
		if bp := firstLaunchableBinaryInPackage(target.Dir, holonDirSlug(target.Dir)); bp != "" {
			return bp
		}
	}
	return ""
}

func binaryLookupNames(target *Target, requested string) []string {
	names := []string{requested}
	if target != nil && target.Manifest != nil {
		names = append(names, target.Manifest.BinaryName())
	}
	if target != nil {
		names = append(names, filepath.Base(target.Dir))
		if target.Identity != nil {
			if idSlug := target.Identity.Slug(); idSlug != "" {
				names = append(names, idSlug)
			}
		}
	}
	return uniqueNonEmpty(names)
}

func lookupBinaryOnSystem(names ...string) string {
	for _, candidate := range uniqueNonEmpty(names) {
		if installed := lookupInstalledLaunchableInOPBIN(candidate); installed != "" {
			return installed
		}
		if path, lookErr := exec.LookPath(candidate); lookErr == nil {
			return path
		}
	}
	return ""
}

func isTargetNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}

func DiscoverInPath() []string {
	names := []string{"op"}

	if holons, err := DiscoverLocalHolons(); err == nil {
		for _, holon := range holons {
			if holon.Manifest != nil && holon.Manifest.BinaryName() != "" {
				names = append(names, holon.Manifest.BinaryName())
				continue
			}
			names = append(names, filepath.Base(holon.Dir))
		}
	}
	if holons, err := DiscoverCachedHolons(); err == nil {
		for _, holon := range holons {
			if holon.Manifest != nil && holon.Manifest.BinaryName() != "" {
				names = append(names, holon.Manifest.BinaryName())
			}
		}
	}

	found := make([]string, 0, len(names))
	for _, name := range uniqueNonEmpty(names) {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if strings.HasPrefix(path, filepath.Clean(openv.OPBIN())+string(os.PathSeparator)) {
			continue
		}
		found = append(found, fmt.Sprintf("%s -> %s", name, path))
	}
	sort.Strings(found)
	return found
}

func DiscoverInOPBIN() []string {
	opbin := openv.OPBIN()
	entries, err := os.ReadDir(opbin)
	if err != nil {
		return nil
	}

	found := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if linkTarget, lErr := os.Readlink(filepath.Join(opbin, name)); lErr == nil {
				if isHolonPackagePath(strings.SplitN(filepath.ToSlash(linkTarget), "/", 2)[0]) {
					continue
				}
			}
		}
		path := filepath.Join(opbin, name)
		if info.IsDir() && !isMacAppBundlePath(name) {
			found = append(found, fmt.Sprintf("%s -> %s", name, path))
			continue
		}
		found = append(found, fmt.Sprintf("%s -> %s", name, path))
	}
	sort.Strings(found)
	return found
}

func pathFromRefURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		return trimmed, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.FromSlash(parsed.Path)), nil
}

func holonDirSlug(dir string) string {
	base := strings.ToLower(filepath.Base(dir))
	if isHolonPackagePath(base) {
		return strings.TrimSuffix(base, ".holon")
	}
	return base
}

func holonRelativePath(root, dir string) string {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	if rel, err := filepath.Rel(root, dir); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(dir)
}

func workspaceRelativePath(path string) string {
	base := workspaceRoot()
	absPath, err := filepath.Abs(path)
	if err == nil {
		if rel, relErr := filepath.Rel(base, absPath); relErr == nil {
			return filepath.ToSlash(rel)
		}
	}
	if rel, err := filepath.Rel(base, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func workspaceRoot() string {
	return openv.Root()
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
