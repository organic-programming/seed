package holons

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/identity"
)

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

func DiscoverHolons(root string) ([]LocalHolon, error) {
	return discoverHolonsInRoot(root, "local", holonRelativePath)
}

func DiscoverLocalHolons() ([]LocalHolon, error) {
	return DiscoverHolons(openv.Root())
}

func DiscoverCachedHolons() ([]LocalHolon, error) {
	cacheDir := openv.CacheDir()
	info, err := os.Stat(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	return discoverHolonsInRoot(cacheDir, "cached", cacheRelativePath)
}

func discoverHolonsInRoot(root, origin string, relPath func(string, string) string) ([]LocalHolon, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = openv.Root()
	}
	absRoot, err := filepath.Abs(root)
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

	candidates := make(map[string]LocalHolon)
	orderedKeys := make([]string, 0)
	protoFiles := make([]string, 0)
	holonPackageDirs := make([]string, 0)

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			// Collect .holon package directories and stop descending into them.
			if path != absRoot && isHolonPackagePath(d.Name()) {
				holonPackageDirs = append(holonPackageDirs, path)
				return filepath.SkipDir
			}
			if shouldSkipDiscoveryDir(absRoot, path, d.Name()) {
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

	// Phase 1: .holon packages (fast path via .holon.json).
	sort.Strings(holonPackageDirs)
	for _, pkgDir := range holonPackageDirs {
		entry, ok := holonFromPackageDir(pkgDir, absRoot, origin, relPath)
		if !ok {
			continue
		}
		key := strings.TrimSpace(entry.Identity.UUID)
		if key == "" {
			key = pkgDir
		}
		candidates[key] = entry
		orderedKeys = append(orderedKeys, key)
	}

	// Phase 2: source holons (holon.proto walk).
	sort.Strings(protoFiles)
	for _, protoPath := range protoFiles {
		resolved, err := identity.ResolveFromProtoFile(protoPath)
		if err != nil {
			continue
		}
		if resolved.Identity.GivenName == "" && resolved.Identity.FamilyName == "" {
			continue
		}

		dir := protoHolonDir(absRoot, protoPath, resolved)
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}

		entry := LocalHolon{
			Dir:          absDir,
			RelativePath: relPath(absRoot, absDir),
			Origin:       origin,
			Identity:     resolved.Identity,
			IdentityPath: resolved.SourcePath,
		}
		if manifest, loadErr := LoadManifest(absDir); loadErr == nil {
			entry.Manifest = manifest
		}

		key := strings.TrimSpace(entry.Identity.UUID)
		if key == "" {
			key = absDir
		}
		if existing, ok := candidates[key]; ok {
			if shouldReplaceDiscoveredHolon(existing, entry) {
				candidates[key] = entry
			}
			continue
		}

		candidates[key] = entry
		orderedKeys = append(orderedKeys, key)
	}

	entries := make([]LocalHolon, 0, len(candidates))
	for _, key := range orderedKeys {
		entry, ok := candidates[key]
		if ok {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RelativePath == entries[j].RelativePath {
			return entries[i].Identity.UUID < entries[j].Identity.UUID
		}
		return entries[i].RelativePath < entries[j].RelativePath
	})
	return entries, nil
}

func shouldSkipDiscoveryDir(root, path, name string) bool {
	if path == root {
		return false
	}
	if name == ".git" || name == ".op" || name == "node_modules" || name == "vendor" || name == "build" || name == "testdata" {
		return true
	}
	// .holon packages are handled before this check — they are collected
	// and skipped separately. Other dot-prefixed dirs are excluded.
	return strings.HasPrefix(name, ".")
}

// holonFromPackageDir reads .holon.json from a .holon package directory
// and returns a LocalHolon entry. Returns false if the package cannot be read.
func holonFromPackageDir(pkgDir, absRoot, origin string, relPath func(string, string) string) (LocalHolon, bool) {
	pkg, err := readHolonPackageJSON(pkgDir)
	if err != nil {
		return LocalHolon{}, false
	}
	if pkg.Identity.GivenName == "" && pkg.Identity.FamilyName == "" {
		return LocalHolon{}, false
	}

	id := identity.Identity{
		Schema:     pkg.Schema,
		UUID:       pkg.UUID,
		GivenName:  pkg.Identity.GivenName,
		FamilyName: pkg.Identity.FamilyName,
		Motto:      pkg.Identity.Motto,
		Status:     pkg.Status,
		Lang:       pkg.Lang,
	}

	return LocalHolon{
		Dir:          pkgDir,
		RelativePath: relPath(absRoot, pkgDir),
		Origin:       origin,
		Identity:     id,
		IdentityPath: filepath.Join(pkgDir, ".holon.json"),
	}, true
}

// readHolonPackageJSON reads and parses .holon.json from a package directory.
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

var protoVersionDirPattern = regexp.MustCompile(`^v[0-9]+(?:[A-Za-z0-9._-]*)?$`)

func shouldReplaceDiscoveredHolon(current, next LocalHolon) bool {
	currentDepth := discoveryPathDepth(current.RelativePath)
	nextDepth := discoveryPathDepth(next.RelativePath)
	if nextDepth < currentDepth {
		return true
	}
	if nextDepth > currentDepth {
		return false
	}
	return isProtoIdentityPath(next.IdentityPath) && !isProtoIdentityPath(current.IdentityPath)
}

func isProtoIdentityPath(path string) bool {
	return filepath.Base(path) == identity.ProtoManifestFileName
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

func ResolveTarget(ref string) (*Target, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		trimmed = "."
	}

	if dir, ok, err := existingTargetDir(trimmed); err != nil {
		return nil, err
	} else if ok {
		return resolveDir(trimmed, dir)
	}

	if target, err := resolveTargetBySlug(trimmed); err == nil {
		return target, nil
	} else if !isTargetNotFound(err) {
		return nil, err
	}

	if target, err := resolveTargetByUUID(trimmed); err == nil {
		return target, nil
	} else if !isTargetNotFound(err) {
		return nil, err
	}

	return nil, fmt.Errorf("holon %q not found", trimmed)
}

func ResolveBinary(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("holon %q not found", name)
	}

	if dir, ok, err := existingTargetDir(trimmed); err != nil {
		return "", err
	} else if ok {
		target, err := resolveDir(trimmed, dir)
		if err != nil {
			return "", err
		}
		if binaryPath := builtBinaryForTarget(target); binaryPath != "" {
			return binaryPath, nil
		}
		if systemPath := lookupBinaryOnSystem(binaryLookupNames(target, trimmed)...); systemPath != "" {
			return systemPath, nil
		}
		return "", fmt.Errorf("holon %q not found", name)
	}

	if target, err := resolveTargetBySlugFromOrigins(trimmed, true, false); err == nil {
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

	if target, err := resolveTargetBySlugFromOrigins(trimmed, false, true); err == nil {
		if binaryPath := builtBinaryForTarget(target); binaryPath != "" {
			return binaryPath, nil
		}
		if systemPath := lookupBinaryOnSystem(binaryLookupNames(target, trimmed)...); systemPath != "" {
			return systemPath, nil
		}
	} else if !isTargetNotFound(err) {
		return "", err
	}

	if target, err := resolveTargetByUUID(trimmed); err == nil {
		if binaryPath := builtBinaryForTarget(target); binaryPath != "" {
			return binaryPath, nil
		}
		if systemPath := lookupBinaryOnSystem(binaryLookupNames(target, trimmed)...); systemPath != "" {
			return systemPath, nil
		}
	} else if !isTargetNotFound(err) {
		return "", err
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

func resolveTargetBySlug(ref string) (*Target, error) {
	return resolveTargetBySlugFromOrigins(ref, true, true)
}

func resolveTargetBySlugFromOrigins(ref string, includeLocal, includeCache bool) (*Target, error) {
	matches, err := collectSlugMatches(ref, includeLocal, includeCache)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("holon %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, ambiguousHolonError(ref, matches)
	}
	return resolveDir(ref, matches[0].Dir)
}

func resolveTargetByUUID(ref string) (*Target, error) {
	matches, err := collectUUIDMatches(ref)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("holon %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, ambiguousHolonError(ref, matches)
	}
	return resolveDir(ref, matches[0].Dir)
}

func collectSlugMatches(ref string, includeLocal, includeCache bool) ([]LocalHolon, error) {
	var combined []LocalHolon
	if includeLocal {
		local, err := DiscoverLocalHolons()
		if err != nil {
			return nil, err
		}
		combined = append(combined, filterHolonsBySlug(local, ref)...)
	}
	if includeCache {
		cached, err := DiscoverCachedHolons()
		if err != nil {
			return nil, err
		}
		combined = append(combined, filterHolonsBySlug(cached, ref)...)
	}
	return collapseMatchesByUUID(combined), nil
}

func collectUUIDMatches(ref string) ([]LocalHolon, error) {
	local, err := DiscoverLocalHolons()
	if err != nil {
		return nil, err
	}
	cached, err := DiscoverCachedHolons()
	if err != nil {
		return nil, err
	}
	combined := append(filterHolonsByUUID(local, ref), filterHolonsByUUID(cached, ref)...)
	return collapseMatchesByUUID(combined), nil
}

func filterHolonsBySlug(holons []LocalHolon, ref string) []LocalHolon {
	lowered := strings.ToLower(strings.TrimSpace(ref))
	matches := make([]LocalHolon, 0)
	for _, holon := range holons {
		if holonDirSlug(holon.Dir) == lowered {
			matches = append(matches, holon)
			continue
		}
		if idSlug := holon.Identity.Slug(); idSlug != "" && idSlug == lowered {
			matches = append(matches, holon)
			continue
		}
		for _, alias := range holon.Identity.Aliases {
			if strings.ToLower(strings.TrimSpace(alias)) == lowered {
				matches = append(matches, holon)
				break
			}
		}
	}
	return matches
}

// holonDirSlug returns the canonical slug from a holon directory path,
// stripping the .holon suffix when present.
func holonDirSlug(dir string) string {
	base := strings.ToLower(filepath.Base(dir))
	if isHolonPackagePath(base) {
		return strings.TrimSuffix(base, ".holon")
	}
	return base
}

func filterHolonsByUUID(holons []LocalHolon, ref string) []LocalHolon {
	trimmed := strings.TrimSpace(ref)
	matches := make([]LocalHolon, 0)
	for _, holon := range holons {
		uuid := strings.TrimSpace(holon.Identity.UUID)
		if uuid == "" {
			continue
		}
		if uuid == trimmed || strings.HasPrefix(uuid, trimmed) {
			matches = append(matches, holon)
		}
	}
	return matches
}

func collapseMatchesByUUID(matches []LocalHolon) []LocalHolon {
	seen := make(map[string]struct{}, len(matches))
	collapsed := make([]LocalHolon, 0, len(matches))
	for _, match := range matches {
		key := strings.TrimSpace(match.Identity.UUID)
		if key == "" {
			key = match.Dir
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		collapsed = append(collapsed, match)
	}
	return collapsed
}

func ambiguousHolonError(ref string, matches []LocalHolon) error {
	var b strings.Builder
	fmt.Fprintf(&b, "ambiguous holon %q — found %d matches (different UUIDs):", ref, len(matches))
	for i, match := range matches {
		fmt.Fprintf(
			&b,
			"\n  %d. [%s]  %s  UUID %s",
			i+1,
			match.Origin,
			disambiguationPath(match),
			match.Identity.UUID,
		)
	}
	fmt.Fprintf(&b, "\nDisambiguate with a path or UUID:")
	for _, match := range matches {
		fmt.Fprintf(&b, "\n  op build %s", disambiguationPath(match))
		fmt.Fprintf(&b, "\n  op build %s", shortUUIDValue(match.Identity.UUID))
	}
	return errors.New(b.String())
}

func disambiguationPath(match LocalHolon) string {
	if match.Origin == "local" {
		rel := filepath.ToSlash(match.RelativePath)
		if rel == "" || rel == "." {
			return "./"
		}
		if strings.HasPrefix(rel, "./") {
			return rel
		}
		return "./" + rel
	}
	return filepath.ToSlash(match.RelativePath)
}

func shortUUIDValue(uuid string) string {
	if len(uuid) <= 8 {
		return uuid
	}
	return uuid[:8]
}

func builtBinaryForTarget(target *Target) string {
	if target == nil {
		return ""
	}
	// Standard manifest path.
	if target.Manifest != nil {
		binaryPath := target.Manifest.BinaryPath()
		if binaryPath != "" {
			if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
				return binaryPath
			}
		}
	}
	// .holon package path: use .holon.json entrypoint + PackageBinaryPath.
	if isHolonPackagePath(target.Dir) {
		if pkg, err := readHolonPackageJSON(target.Dir); err == nil && pkg.Entrypoint != "" {
			if bp := PackageBinaryPath(target.Dir, pkg.Entrypoint); bp != "" {
				if info, err := os.Stat(bp); err == nil && !info.IsDir() {
					return bp
				}
			}
		}
		// Fallback: probe any binary in the package.
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
		// Skip symlinks that point into a .holon package (e.g. op → grace-op.holon/…).
		if entry.Type()&os.ModeSymlink != 0 {
			if linkTarget, lErr := os.Readlink(filepath.Join(opbin, name)); lErr == nil {
				if isHolonPackagePath(strings.SplitN(filepath.ToSlash(linkTarget), "/", 2)[0]) {
					continue
				}
			}
		}
		if info.IsDir() && !isMacAppBundlePath(name) {
			path := filepath.Join(opbin, name)
			found = append(found, fmt.Sprintf("%s -> %s", name, path))
			continue
		}
		path := filepath.Join(opbin, name)
		found = append(found, fmt.Sprintf("%s -> %s", name, path))
	}
	sort.Strings(found)
	return found
}

func resolveDir(ref, dir string) (*Target, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	target := &Target{
		Ref:          ref,
		Dir:          absDir,
		RelativePath: workspaceRelativePath(absDir),
	}

	if resolved, resolveErr := identity.Resolve(absDir); resolveErr == nil {
		target.Identity = &resolved.Identity
		target.IdentityPath = resolved.SourcePath
	}

	manifest, loadErr := LoadManifest(absDir)
	if loadErr != nil {
		if target.IdentityPath != "" || !errors.Is(loadErr, errProtoManifestNotFound) {
			target.ManifestErr = loadErr
		}
		return target, nil
	}

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

func existingTargetDir(ref string) (string, bool, error) {
	info, err := os.Stat(ref)
	if err != nil {
		if os.IsNotExist(err) {
			// Try <ref>.holon as a package directory.
			pkgPath := ref + ".holon"
			if pkgInfo, pkgErr := os.Stat(pkgPath); pkgErr == nil && pkgInfo.IsDir() {
				return pkgPath, true, nil
			}
			return "", false, nil
		}
		return "", false, err
	}

	if info.IsDir() {
		return ref, true, nil
	}

	switch filepath.Base(ref) {
	case identity.ProtoManifestFileName:
		return filepath.Dir(ref), true, nil
	default:
		return "", false, fmt.Errorf("%s is not a holon directory", ref)
	}
}

func holonRelativePath(root, dir string) string {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	if rel, err := filepath.Rel(root, dir); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(dir)
}

func cacheRelativePath(root, dir string) string {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	if rel, err := filepath.Rel(root, dir); err == nil {
		if rel == "." {
			return "."
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return filepath.ToSlash(rel)
		}
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

func hasKnownRoot(base string) bool {
	return filepath.Clean(base) == filepath.Clean(openv.Root())
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
