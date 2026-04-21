package holons

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type HolonPackageJSON struct {
	Schema        string            `json:"schema"`
	Slug          string            `json:"slug"`
	UUID          string            `json:"uuid"`
	Aliases       []string          `json:"aliases,omitempty"`
	Identity      HolonIdentityJSON `json:"identity"`
	Lang          string            `json:"lang"`
	Runner        string            `json:"runner"`
	Status        string            `json:"status"`
	Kind          string            `json:"kind"`
	Transport     string            `json:"transport"`
	Entrypoint    string            `json:"entrypoint"`
	Architectures []string          `json:"architectures"`
	Hardened      bool              `json:"hardened"`
	Standalone    bool              `json:"standalone"`
	HasDist       bool              `json:"has_dist"`
	HasSource     bool              `json:"has_source"`
}

type HolonIdentityJSON struct {
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Motto      string `json:"motto"`
}

func shouldWriteHolonJSON(manifest *LoadedManifest) bool {
	if manifest == nil || manifest.Manifest.Kind == KindComposite || manifestHasPrimaryArtifact(manifest) {
		return false
	}
	return strings.TrimSpace(manifest.BinaryName()) != ""
}

func writeHolonJSON(manifest *LoadedManifest, ctx BuildContext) error {
	if !shouldWriteHolonJSON(manifest) {
		return nil
	}

	pkgDir := manifest.HolonPackageDir()
	if pkgDir == "" {
		return fmt.Errorf("holon package directory is not available")
	}
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return err
	}

	payload := HolonPackageJSON{
		Schema: "holon-package/v1",
		Slug:   manifest.Name,
		UUID:   strings.TrimSpace(manifest.Manifest.UUID),
		Aliases: append([]string(nil),
			manifest.Manifest.Aliases...,
		),
		Identity: HolonIdentityJSON{
			GivenName:  strings.TrimSpace(manifest.Manifest.GivenName),
			FamilyName: strings.TrimSpace(manifest.Manifest.FamilyName),
			Motto:      strings.TrimSpace(manifest.Manifest.Motto),
		},
		Lang:          strings.TrimSpace(manifest.Manifest.Lang),
		Runner:        strings.TrimSpace(manifest.Manifest.Build.Runner),
		Status:        strings.TrimSpace(manifest.Manifest.Status),
		Kind:          strings.TrimSpace(manifest.Manifest.Kind),
		Transport:     strings.TrimSpace(manifest.Manifest.Transport),
		Entrypoint:    strings.TrimSpace(manifest.BinaryName()),
		Architectures: packageArchitectures(pkgDir),
		Hardened:      ctx.Hardened,
		Standalone:    runnerProducesStandaloneArtifact(manifest.Manifest.Build.Runner),
		HasDist:       dirExists(filepath.Join(pkgDir, "dist")),
		HasSource:     dirExists(filepath.Join(pkgDir, "git")),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(pkgDir, ".holon.json"), data, 0o644)
}

func packageArchitectures(pkgDir string) []string {
	entries, err := os.ReadDir(filepath.Join(pkgDir, "bin"))
	if err != nil {
		return nil
	}

	architectures := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			architectures = append(architectures, entry.Name())
		}
	}
	sort.Strings(architectures)
	return architectures
}

// writeHolonJSONForInstall writes .holon.json into pkgDir for any holon
// (including composites). Used at install time when wrapping raw artifacts
// into a .holon package.
func writeHolonJSONForInstall(manifest *LoadedManifest, pkgDir string) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	entrypoint := strings.TrimSpace(manifest.BinaryName())
	if entrypoint == "" {
		entrypoint = strings.TrimSpace(manifest.Manifest.ArtifactPath())
		if entrypoint != "" {
			entrypoint = filepath.Base(entrypoint)
		}
	}

	payload := HolonPackageJSON{
		Schema: "holon-package/v1",
		Slug:   manifest.Name,
		UUID:   strings.TrimSpace(manifest.Manifest.UUID),
		Aliases: append([]string(nil),
			manifest.Manifest.Aliases...,
		),
		Identity: HolonIdentityJSON{
			GivenName:  strings.TrimSpace(manifest.Manifest.GivenName),
			FamilyName: strings.TrimSpace(manifest.Manifest.FamilyName),
			Motto:      strings.TrimSpace(manifest.Manifest.Motto),
		},
		Lang:          strings.TrimSpace(manifest.Manifest.Lang),
		Runner:        strings.TrimSpace(manifest.Manifest.Build.Runner),
		Status:        strings.TrimSpace(manifest.Manifest.Status),
		Kind:          strings.TrimSpace(manifest.Manifest.Kind),
		Transport:     strings.TrimSpace(manifest.Manifest.Transport),
		Entrypoint:    entrypoint,
		Architectures: packageArchitectures(pkgDir),
		Hardened:      false,
		Standalone:    runnerProducesStandaloneArtifact(manifest.Manifest.Build.Runner),
		HasDist:       dirExists(filepath.Join(pkgDir, "dist")),
		HasSource:     dirExists(filepath.Join(pkgDir, "git")),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(pkgDir, ".holon.json"), data, 0o644)
}

func updateArtifactHolonJSON(manifest *LoadedManifest, ctx BuildContext) error {
	if manifest == nil {
		return nil
	}
	for _, pkgDir := range []string{manifest.ArtifactPath(ctx), manifest.HolonPackageDir()} {
		if pkgDir == "" {
			continue
		}
		if err := updateHolonJSONHardened(pkgDir, ctx.Hardened); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func updateHolonJSONHardened(pkgDir string, hardened bool) error {
	holonJSONPath := pkgDir
	if info, err := os.Stat(pkgDir); err == nil && info.IsDir() {
		holonJSONPath = filepath.Join(pkgDir, ".holon.json")
	} else if filepath.Base(pkgDir) != ".holon.json" {
		return os.ErrNotExist
	}

	data, err := os.ReadFile(holonJSONPath)
	if err != nil {
		return err
	}

	var payload HolonPackageJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	payload.Hardened = hardened

	data, err = json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(holonJSONPath, data, 0o644)
}
