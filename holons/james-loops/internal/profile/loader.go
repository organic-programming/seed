package profile

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	rootpkg "github.com/organic-programming/james-loops"
	"gopkg.in/yaml.v3"
)

const bundledProfilesDir = ".op/profiles"

var (
	repoRootResolver = discoverRepoRoot
	userHomeResolver = os.UserHomeDir
)

// Load returns the Profile for the given name.
// Returns an error with a hint pointing to `james-loops profile list`
// if the profile is not found in any location.
func Load(name string) (Profile, error) {
	target := profileFileName(name)
	for _, source := range loadSources() {
		profile, ok, err := source.load(target)
		if err != nil {
			return Profile{}, err
		}
		if ok {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf(
		"profile %q not found\n(run `james-loops profile list` to see available profiles)",
		strings.TrimSuffix(target, ".yaml"),
	)
}

// LoadAll returns all profiles from all sources (dedup by name, first wins).
func LoadAll() ([]Profile, error) {
	seen := make(map[string]struct{})
	var profiles []Profile
	for _, source := range loadSources() {
		items, err := source.loadAll()
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if _, ok := seen[item.Name]; ok {
				continue
			}
			seen[item.Name] = struct{}{}
			profiles = append(profiles, item)
		}
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

type source struct {
	load    func(name string) (Profile, bool, error)
	loadAll func() ([]Profile, error)
}

func loadSources() []source {
	localRoot, _ := resolveLocalProfilesRoot()
	userRoot, _ := resolveUserProfilesRoot()
	return []source{
		fsSource(os.DirFS(localRoot), ".", localRoot),
		fsSource(rootpkg.BundledProfilesFS, bundledProfilesDir, "embedded:.op/profiles"),
		fsSource(os.DirFS(userRoot), ".", userRoot),
	}
}

func fsSource(filesystem fs.FS, dir string, label string) source {
	return source{
		load: func(name string) (Profile, bool, error) {
			path := joinProfilePath(dir, name)
			profile, err := readProfileFS(filesystem, path, name)
			if err == nil {
				return profile, true, nil
			}
			if isNotExist(err) {
				return Profile{}, false, nil
			}
			return Profile{}, false, fmt.Errorf("load profile %s from %s: %w", name, label, err)
		},
		loadAll: func() ([]Profile, error) {
			entries, err := fs.ReadDir(filesystem, dir)
			if err != nil {
				if isNotExist(err) {
					return nil, nil
				}
				return nil, fmt.Errorf("read profiles from %s: %w", label, err)
			}
			var profiles []Profile
			for _, entry := range entries {
				if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
					continue
				}
				profile, err := readProfileFS(filesystem, joinProfilePath(dir, entry.Name()), entry.Name())
				if err != nil {
					return nil, fmt.Errorf("load profile %s from %s: %w", entry.Name(), label, err)
				}
				profiles = append(profiles, profile)
			}
			sort.Slice(profiles, func(i, j int) bool {
				return profiles[i].Name < profiles[j].Name
			})
			return profiles, nil
		},
	}
}

func readProfileFS(filesystem fs.FS, path string, fileName string) (Profile, error) {
	data, err := fs.ReadFile(filesystem, path)
	if err != nil {
		return Profile{}, err
	}
	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	if profile.Name == "" {
		profile.Name = strings.TrimSuffix(fileName, ".yaml")
	}
	return profile, nil
}

func resolveLocalProfilesRoot() (string, error) {
	repoRoot, err := repoRootResolver()
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, "ader", "loops", "profiles"), nil
}

func resolveUserProfilesRoot() (string, error) {
	home, err := userHomeResolver()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".james-loops", "profiles"), nil
}

func discoverRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		return "", wdErr
	}
	return wd, nil
}

func profileFileName(name string) string {
	trimmed := strings.TrimSpace(name)
	if filepath.Ext(trimmed) != ".yaml" {
		trimmed += ".yaml"
	}
	return trimmed
}

func joinProfilePath(dir string, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return filepath.ToSlash(filepath.Join(dir, name))
}

func isNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
