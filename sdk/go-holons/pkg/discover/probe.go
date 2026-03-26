package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ProbeFunc is called when a .holon package directory has no .holon.json.
// It receives the absolute package dir path and should return a HolonEntry
// if the holon can be probed (e.g. via stdio Describe). It may also write
// .holon.json as a side effect using WritePackageJSON.
type ProbeFunc func(packageDir string) (*HolonEntry, error)

var (
	probeMu sync.RWMutex
	probeFn ProbeFunc
)

// SetProbe registers an optional fallback probe for packages missing .holon.json.
// Callers (e.g. grace-op CLI) can set a probe that launches the holon binary
// via stdio, calls HolonMeta.Describe, and returns the result.
func SetProbe(fn ProbeFunc) {
	probeMu.Lock()
	defer probeMu.Unlock()
	probeFn = fn
}

func getProbe() ProbeFunc {
	probeMu.RLock()
	defer probeMu.RUnlock()
	return probeFn
}

// WritePackageJSON writes a .holon.json cache file inside the given package directory.
// The entry's fields are projected into the holon-package/v1 schema.
func WritePackageJSON(packageDir string, entry HolonEntry) error {
	payload := holonPackageJSON{
		Schema: "holon-package/v1",
		Slug:   entry.Slug,
		UUID:   entry.UUID,
		Identity: holonIdentityJSON{
			GivenName:  entry.Identity.GivenName,
			FamilyName: entry.Identity.FamilyName,
			Motto:      entry.Identity.Motto,
		},
		Lang:          entry.Identity.Lang,
		Status:        entry.Identity.Status,
		Entrypoint:    entry.Entrypoint,
		Architectures: entry.Architectures,
		HasDist:       entry.HasDist,
		HasSource:     entry.HasSource,
	}

	if entry.Manifest != nil {
		payload.Runner = entry.Manifest.Build.Runner
		payload.Kind = entry.Manifest.Kind
	}
	if payload.Runner == "" {
		payload.Runner = entry.Runner
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	target := filepath.Join(packageDir, ".holon.json")
	return os.WriteFile(target, append(data, '\n'), 0o644)
}

// probePackageEntry attempts to resolve a .holon package dir that has no
// .holon.json by calling the registered probe function.
func probePackageEntry(root, dir, origin string) (HolonEntry, error) {
	probe := getProbe()
	if probe == nil {
		return HolonEntry{}, os.ErrNotExist
	}

	entry, err := probe(dir)
	if err != nil || entry == nil {
		if err == nil {
			err = os.ErrNotExist
		}
		return HolonEntry{}, err
	}

	absRoot, _ := filepath.Abs(strings.TrimSpace(root))
	absDir, _ := filepath.Abs(dir)
	entry.Dir = absDir
	entry.RelativePath = relativePath(absRoot, absDir)
	entry.Origin = origin
	entry.PackageRoot = absDir

	return *entry, nil
}
