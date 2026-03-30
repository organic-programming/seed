package mod

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/modfile"
	"github.com/organic-programming/grace-op/internal/progress"
)

type Dependency struct {
	Path      string `json:"path"`
	Version   string `json:"version"`
	CachePath string `json:"cache_path,omitempty"`
}

type Edge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Version string `json:"version"`
}

type UpdatedDependency struct {
	Path       string `json:"path"`
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`
}

type InitResult struct {
	ModFile   string `json:"mod_file"`
	HolonPath string `json:"holon_path"`
}

type AddResult struct {
	Dependency Dependency `json:"dependency"`
	Deferred   bool       `json:"deferred,omitempty"`
}

type RemoveResult struct {
	Path string `json:"path"`
}

type PullResult struct {
	Fetched []Dependency `json:"fetched"`
}

type UpdateResult struct {
	Updated []UpdatedDependency `json:"updated"`
}

type ListResult struct {
	HolonPath    string       `json:"holon_path"`
	Dependencies []Dependency `json:"dependencies"`
}

type GraphResult struct {
	Root  string `json:"root"`
	Edges []Edge `json:"edges"`
}

type TidyResult struct {
	SumFile string       `json:"sum_file"`
	Pruned  []string     `json:"pruned,omitempty"`
	Current []Dependency `json:"current"`
}

type Options struct {
	Progress progress.Reporter
}

var listRemoteTags = realListRemoteTags

func SetRemoteTagsForTesting(fn func(string) ([]string, error)) func() {
	previous := listRemoteTags
	if fn == nil {
		listRemoteTags = realListRemoteTags
	} else {
		listRemoteTags = fn
	}
	return func() {
		listRemoteTags = previous
	}
}

func Init(dir, holonPath string) (*InitResult, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	modPath := filepath.Join(dir, "holon.mod")
	if _, err := os.Stat(modPath); err == nil {
		return nil, fmt.Errorf("holon.mod already exists in %s", dir)
	}

	resolvedPath, err := resolveHolonPath(dir, holonPath)
	if err != nil {
		return nil, err
	}

	mod := &modfile.ModFile{HolonPath: resolvedPath}
	if err := mod.Write(modPath); err != nil {
		return nil, err
	}

	return &InitResult{
		ModFile:   modPath,
		HolonPath: resolvedPath,
	}, nil
}

func Add(dir, depPath, version string) (*AddResult, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	depPath = strings.TrimSpace(depPath)
	if depPath == "" {
		return nil, fmt.Errorf("dependency path is required")
	}

	if strings.TrimSpace(version) == "" {
		latest, err := latestAvailableTag(depPath)
		if err != nil {
			return nil, fmt.Errorf("resolve latest version for %s: %w", depPath, err)
		}
		version = latest
	}

	modPath := filepath.Join(dir, "holon.mod")
	mod, err := modfile.Parse(modPath)
	if err != nil {
		return nil, fmt.Errorf("parse holon.mod: %w", err)
	}

	mod.AddRequire(depPath, version)
	if err := mod.Write(modPath); err != nil {
		return nil, fmt.Errorf("write holon.mod: %w", err)
	}

	cachePath, fetchErr := fetchToCache(depPath, version)
	deferred := false
	if fetchErr != nil {
		cachePath = ""
		deferred = true
	} else if err := updateSumEntry(dir, depPath, version, cachePath); err != nil {
		return nil, err
	}

	return &AddResult{
		Dependency: Dependency{
			Path:      depPath,
			Version:   version,
			CachePath: cachePath,
		},
		Deferred: deferred,
	}, nil
}

func Remove(dir, depPath string) (*RemoveResult, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	depPath = strings.TrimSpace(depPath)
	if depPath == "" {
		return nil, fmt.Errorf("dependency path is required")
	}

	modPath := filepath.Join(dir, "holon.mod")
	mod, err := modfile.Parse(modPath)
	if err != nil {
		return nil, fmt.Errorf("parse holon.mod: %w", err)
	}

	if !mod.RemoveRequire(depPath) {
		return nil, fmt.Errorf("dependency %q not found in holon.mod", depPath)
	}
	if err := mod.Write(modPath); err != nil {
		return nil, fmt.Errorf("write holon.mod: %w", err)
	}

	return &RemoveResult{Path: depPath}, nil
}

func Pull(dir string, opts ...Options) (*PullResult, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	reporter := modProgress(opts)
	reporter.Step("resolving versions...")

	mod, _, err := loadMod(dir)
	if err != nil {
		return nil, err
	}

	sumPath := filepath.Join(dir, "holon.sum")
	sum, err := modfile.ParseSum(sumPath)
	if err != nil {
		return nil, fmt.Errorf("parse holon.sum: %w", err)
	}

	fetched := make([]Dependency, 0, len(mod.Require))
	for _, req := range mod.Require {
		if mod.ResolvedPath(req.Path) != "" {
			continue
		}
		reporter.Step(fmt.Sprintf("fetching %s@%s...", req.Path, req.Version))
		cachePath, err := fetchToCache(req.Path, req.Version)
		if err != nil {
			return nil, fmt.Errorf("fetch %s@%s: %w", req.Path, req.Version, err)
		}
		if err := setSumHashes(sum, req.Path, req.Version, cachePath); err != nil {
			return nil, err
		}
		fetched = append(fetched, Dependency{
			Path:      req.Path,
			Version:   req.Version,
			CachePath: cachePath,
		})
	}

	if err := sum.Write(sumPath); err != nil {
		return nil, fmt.Errorf("write holon.sum: %w", err)
	}
	return &PullResult{Fetched: fetched}, nil
}

func Update(dir, targetModule string, opts ...Options) (*UpdateResult, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	reporter := modProgress(opts)
	reporter.Step("resolving latest versions...")

	mod, modPath, err := loadMod(dir)
	if err != nil {
		return nil, err
	}

	targetModule = strings.TrimSpace(targetModule)
	updated := make([]UpdatedDependency, 0, len(mod.Require))
	for i, dep := range mod.Require {
		if targetModule != "" && dep.Path != targetModule {
			continue
		}
		if mod.ResolvedPath(dep.Path) != "" {
			continue
		}

		latest, err := latestCompatibleTag(dep.Path, dep.Version)
		if err != nil {
			return nil, fmt.Errorf("update %s: %w", dep.Path, err)
		}
		if latest == dep.Version {
			continue
		}

		oldCache := cachePathFor(dep.Path, dep.Version)
		_ = os.RemoveAll(oldCache)
		mod.Require[i].Version = latest
		updated = append(updated, UpdatedDependency{
			Path:       dep.Path,
			OldVersion: dep.Version,
			NewVersion: latest,
		})
	}

	if len(updated) > 0 {
		reporter.Step("updating holon.mod...")
		if err := mod.Write(modPath); err != nil {
			return nil, fmt.Errorf("write holon.mod: %w", err)
		}
	}

	return &UpdateResult{Updated: updated}, nil
}

func List(dir string) (*ListResult, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}

	mod, _, err := loadMod(dir)
	if err != nil {
		return nil, err
	}

	deps := make([]Dependency, 0, len(mod.Require))
	for _, req := range mod.Require {
		deps = append(deps, Dependency{
			Path:      req.Path,
			Version:   req.Version,
			CachePath: cachePathFor(req.Path, req.Version),
		})
	}
	return &ListResult{
		HolonPath:    mod.HolonPath,
		Dependencies: deps,
	}, nil
}

func Graph(dir string) (*GraphResult, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}

	mod, _, err := loadMod(dir)
	if err != nil {
		return nil, err
	}

	var edges []Edge
	for _, req := range mod.Require {
		edges = append(edges, Edge{
			From:    mod.HolonPath,
			To:      req.Path,
			Version: req.Version,
		})

		cacheMod, err := modfile.Parse(filepath.Join(cachePathFor(req.Path, req.Version), "holon.mod"))
		if err != nil {
			continue
		}
		for _, sub := range cacheMod.Require {
			edges = append(edges, Edge{
				From:    req.Path,
				To:      sub.Path,
				Version: sub.Version,
			})
		}
	}

	return &GraphResult{
		Root:  mod.HolonPath,
		Edges: edges,
	}, nil
}

func Tidy(dir string, opts ...Options) (*TidyResult, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	reporter := modProgress(opts)
	reporter.Step("scanning imports...")

	mod, modPath, err := loadMod(dir)
	if err != nil {
		return nil, err
	}
	mod.Require = canonicalRequires(mod.Require)
	mod.Replace = canonicalReplaces(mod.Replace)
	reporter.Step("cleaning holon.mod...")
	if err := mod.Write(modPath); err != nil {
		return nil, fmt.Errorf("write holon.mod: %w", err)
	}

	sumPath := filepath.Join(dir, "holon.sum")
	reporter.Step("regenerating holon.sum...")
	sum, err := modfile.ParseSum(sumPath)
	if err != nil {
		return nil, fmt.Errorf("parse holon.sum: %w", err)
	}

	required := make(map[string]string, len(mod.Require))
	current := make([]Dependency, 0, len(mod.Require))
	for _, req := range mod.Require {
		required[req.Path] = req.Version
		current = append(current, Dependency{
			Path:      req.Path,
			Version:   req.Version,
			CachePath: cachePathFor(req.Path, req.Version),
		})
	}

	pruned := make([]string, 0)
	kept := make([]modfile.SumEntry, 0, len(sum.Entries))
	for _, entry := range sum.Entries {
		if requiredVersion, ok := required[entry.Path]; ok && requiredVersion == entry.Version {
			kept = append(kept, entry)
			continue
		}
		pruned = append(pruned, entry.Path+" "+entry.Version)
	}
	sum.Entries = kept

	for _, req := range mod.Require {
		cachePath := cachePathFor(req.Path, req.Version)
		if info, err := os.Stat(cachePath); err == nil && info.IsDir() {
			if err := setSumHashes(sum, req.Path, req.Version, cachePath); err != nil {
				return nil, err
			}
		}
	}

	if err := sum.Write(sumPath); err != nil {
		return nil, fmt.Errorf("write holon.sum: %w", err)
	}

	return &TidyResult{
		SumFile: sumPath,
		Pruned:  pruned,
		Current: current,
	}, nil
}

func canonicalRequires(reqs []modfile.Require) []modfile.Require {
	if len(reqs) == 0 {
		return nil
	}

	best := make(map[string]string, len(reqs))
	for _, req := range reqs {
		path := strings.TrimSpace(req.Path)
		version := strings.TrimSpace(req.Version)
		if path == "" || version == "" {
			continue
		}
		existing, ok := best[path]
		if !ok || compareVersions(version, existing) > 0 {
			best[path] = version
		}
	}

	out := make([]modfile.Require, 0, len(best))
	for path, version := range best {
		out = append(out, modfile.Require{Path: path, Version: version})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func canonicalReplaces(repls []modfile.Replace) []modfile.Replace {
	if len(repls) == 0 {
		return nil
	}

	latest := make(map[string]string, len(repls))
	for _, repl := range repls {
		old := strings.TrimSpace(repl.Old)
		localPath := strings.TrimSpace(repl.LocalPath)
		if old == "" || localPath == "" {
			continue
		}
		latest[old] = localPath
	}

	out := make([]modfile.Replace, 0, len(latest))
	for old, localPath := range latest {
		out = append(out, modfile.Replace{Old: old, LocalPath: localPath})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Old < out[j].Old })
	return out
}

func compareVersions(a, b string) int {
	if _, _, _, ok := parseSemver(a); ok {
		if _, _, _, ok := parseSemver(b); ok {
			return compareSemver(a, b)
		}
		return 1
	}
	if _, _, _, ok := parseSemver(b); ok {
		return -1
	}
	return strings.Compare(a, b)
}

func modProgress(opts []Options) progress.Reporter {
	if len(opts) > 0 && opts[0].Progress != nil {
		return opts[0].Progress
	}
	return progress.Silence()
}

func loadMod(dir string) (*modfile.ModFile, string, error) {
	modPath := filepath.Join(dir, "holon.mod")
	mod, err := modfile.Parse(modPath)
	if err != nil {
		return nil, "", fmt.Errorf("parse holon.mod: %w", err)
	}
	return mod, modPath, nil
}

func resolveHolonPath(dir, explicit string) (string, error) {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed, nil
	}

	if resolved, err := identity.Resolve(dir); err == nil {
		if slug := slugForIdentity(resolved.Identity); slug != "" {
			return slug, nil
		}
	}

	base := strings.TrimSpace(filepath.Base(filepath.Clean(dir)))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "", fmt.Errorf("cannot infer holon path for %s", dir)
	}
	return base, nil
}

func slugForIdentity(id identity.Identity) string {
	given := strings.TrimSpace(id.GivenName)
	family := strings.TrimSpace(strings.TrimSuffix(id.FamilyName, "?"))
	if given == "" && family == "" {
		return ""
	}
	slug := strings.TrimSpace(given + "-" + family)
	slug = strings.ToLower(slug)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.Trim(slug, "-")
	return slug
}

func updateSumEntry(dir, depPath, version, cachePath string) error {
	sumPath := filepath.Join(dir, "holon.sum")
	sum, err := modfile.ParseSum(sumPath)
	if err != nil {
		return fmt.Errorf("parse holon.sum: %w", err)
	}
	if err := setSumHashes(sum, depPath, version, cachePath); err != nil {
		return err
	}
	if err := sum.Write(sumPath); err != nil {
		return fmt.Errorf("write holon.sum: %w", err)
	}
	return nil
}

func setSumHashes(sum *modfile.SumFile, depPath, version, cachePath string) error {
	hash, err := hashDir(cachePath)
	if err != nil {
		return fmt.Errorf("hash %s: %w", cachePath, err)
	}
	if hash != "" {
		sum.Set(depPath, version, "h1:"+hash)
	}
	return nil
}

func cachePathFor(depPath, version string) string {
	return filepath.Join(openv.CacheDir(), depPath+"@"+version)
}

func fetchToCache(depPath, version string) (string, error) {
	cachePath := cachePathFor(depPath, version)
	if info, err := os.Stat(cachePath); err == nil && info.IsDir() {
		return cachePath, nil
	}

	gitURL := "https://" + depPath + ".git"
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	if err := cloneTag(gitURL, version, cachePath); err != nil {
		fallbackURL := "https://" + depPath
		if fallbackErr := cloneTag(fallbackURL, version, cachePath); fallbackErr != nil {
			_ = os.RemoveAll(cachePath)
			return "", fmt.Errorf("git clone %s@%s: %w", depPath, version, err)
		}
	}

	_ = os.RemoveAll(filepath.Join(cachePath, ".git"))
	return cachePath, nil
}

func cloneTag(gitURL, version, cachePath string) error {
	cmd := exec.Command("git", "clone", "--depth=1", "--branch", version, gitURL, cachePath)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func hashDir(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		_, _ = h.Write([]byte(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func latestAvailableTag(depPath string) (string, error) {
	tags, err := listRemoteTags(depPath)
	if err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}
	sort.Slice(tags, func(i, j int) bool {
		return compareSemver(tags[i], tags[j]) < 0
	})
	return tags[len(tags)-1], nil
}

func latestCompatibleTag(depPath, currentVersion string) (string, error) {
	tags, err := listRemoteTags(depPath)
	if err != nil {
		return "", err
	}

	currentMajor, _, _, ok := parseSemver(currentVersion)
	if !ok {
		return currentVersion, nil
	}

	candidates := make([]string, 0, len(tags))
	for _, tag := range tags {
		major, _, _, ok := parseSemver(tag)
		if ok && major == currentMajor {
			candidates = append(candidates, tag)
		}
	}
	if len(candidates) == 0 {
		return currentVersion, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return compareSemver(candidates[i], candidates[j]) < 0
	})
	return candidates[len(candidates)-1], nil
}

func realListRemoteTags(depPath string) ([]string, error) {
	for _, gitURL := range []string{
		"https://" + depPath + ".git",
		"https://" + depPath,
	} {
		cmd := exec.Command("git", "ls-remote", "--tags", "--refs", gitURL)
		out, err := cmd.Output()
		if err != nil {
			continue
		}

		var tags []string
		for _, line := range strings.Split(string(out), "\n") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			tag := strings.TrimPrefix(parts[1], "refs/tags/")
			if _, _, _, ok := parseSemver(tag); ok {
				tags = append(tags, tag)
			}
		}
		return tags, nil
	}
	return nil, fmt.Errorf("ls-remote %s failed", depPath)
}

func parseSemver(v string) (major, minor, patch int, ok bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	_, err1 := fmt.Sscan(parts[0], &major)
	_, err2 := fmt.Sscan(parts[1], &minor)
	_, err3 := fmt.Sscan(parts[2], &patch)
	return major, minor, patch, err1 == nil && err2 == nil && err3 == nil
}

func compareSemver(a, b string) int {
	ma, mia, pa, _ := parseSemver(a)
	mb, mib, pb, _ := parseSemver(b)
	if ma != mb {
		return ma - mb
	}
	if mia != mib {
		return mia - mib
	}
	return pa - pb
}
