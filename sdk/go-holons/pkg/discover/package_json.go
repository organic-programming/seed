package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/organic-programming/go-holons/pkg/identity"
)

type holonPackageJSON struct {
	Schema        string            `json:"schema"`
	Slug          string            `json:"slug"`
	UUID          string            `json:"uuid"`
	Identity      holonIdentityJSON `json:"identity"`
	Lang          string            `json:"lang"`
	Runner        string            `json:"runner"`
	Status        string            `json:"status"`
	Kind          string            `json:"kind"`
	Transport     string            `json:"transport"`
	Entrypoint    string            `json:"entrypoint"`
	Architectures []string          `json:"architectures"`
	HasDist       bool              `json:"has_dist"`
	HasSource     bool              `json:"has_source"`
}

type holonIdentityJSON struct {
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Motto      string `json:"motto"`
}

func discoverPackagesDirect(root, origin string) ([]HolonEntry, error) {
	dirs, err := packageDirsDirect(root)
	if err != nil {
		return nil, err
	}
	return discoverPackagesFromDirs(root, origin, dirs)
}

func discoverPackagesRecursive(root, origin string) ([]HolonEntry, error) {
	dirs, err := packageDirsRecursive(root)
	if err != nil {
		return nil, err
	}
	return discoverPackagesFromDirs(root, origin, dirs)
}

func discoverPackagesFromDirs(root, origin string, dirs []string) ([]HolonEntry, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		trimmed = currentRoot()
	}
	absRoot, err := filepath.Abs(trimmed)
	if err != nil {
		return nil, err
	}

	entriesByKey := make(map[string]HolonEntry)
	keys := make([]string, 0, len(dirs))

	for _, dir := range dirs {
		entry, loadErr := loadPackageEntry(absRoot, dir, origin)
		if loadErr != nil {
			// Fallback: probe the holon via registered ProbeFunc (e.g. describe-by-stdio).
			probed, probeErr := probePackageEntry(absRoot, dir, origin)
			if probeErr != nil {
				continue
			}
			entry = probed
		}

		key := entryKey(entry)
		if existing, ok := entriesByKey[key]; ok {
			if shouldReplaceEntry(existing, entry) {
				entriesByKey[key] = entry
			}
			continue
		}

		entriesByKey[key] = entry
		keys = append(keys, key)
	}

	entries := make([]HolonEntry, 0, len(keys))
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

func packageDirsDirect(root string) ([]string, error) {
	absRoot, err := filepath.Abs(strings.TrimSpace(root))
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

	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".holon") {
			continue
		}
		dirs = append(dirs, filepath.Join(absRoot, entry.Name()))
	}
	sort.Strings(dirs)
	return dirs, nil
}

func packageDirsRecursive(root string) ([]string, error) {
	absRoot, err := filepath.Abs(strings.TrimSpace(root))
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

	var dirs []string
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if filepath.Clean(path) == filepath.Clean(absRoot) {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".holon") {
			dirs = append(dirs, path)
			return filepath.SkipDir
		}
		if shouldSkipDir(absRoot, path, d.Name()) {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

func loadPackageEntry(root, dir, origin string) (HolonEntry, error) {
	manifestPath := filepath.Join(dir, ".holon.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return HolonEntry{}, err
	}

	var payload holonPackageJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return HolonEntry{}, err
	}
	if schema := strings.TrimSpace(payload.Schema); schema != "" && schema != "holon-package/v1" {
		return HolonEntry{}, os.ErrInvalid
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return HolonEntry{}, err
	}

	holonIdentity := identity.Identity{
		UUID:       strings.TrimSpace(payload.UUID),
		GivenName:  strings.TrimSpace(payload.Identity.GivenName),
		FamilyName: strings.TrimSpace(payload.Identity.FamilyName),
		Motto:      strings.TrimSpace(payload.Identity.Motto),
		Status:     strings.TrimSpace(payload.Status),
		Lang:       strings.TrimSpace(payload.Lang),
	}

	slug := strings.TrimSpace(payload.Slug)
	if slug == "" {
		slug = holonIdentity.Slug()
	}
	entrypoint := strings.TrimSpace(payload.Entrypoint)
	entry := HolonEntry{
		Slug:         slug,
		UUID:         holonIdentity.UUID,
		Dir:          absDir,
		RelativePath: relativePath(root, absDir),
		Origin:       origin,
		Identity:     holonIdentity,
		Manifest: &Manifest{
			Kind: strings.TrimSpace(payload.Kind),
			Build: Build{
				Runner: strings.TrimSpace(payload.Runner),
			},
			Artifacts: Artifacts{
				Binary: entrypoint,
			},
		},
		SourceKind:    "package",
		PackageRoot:   absDir,
		Runner:        strings.TrimSpace(payload.Runner),
		Entrypoint:    entrypoint,
		Architectures: append([]string(nil), payload.Architectures...),
		HasDist:       payload.HasDist,
		HasSource:     payload.HasSource,
	}
	return entry, nil
}
