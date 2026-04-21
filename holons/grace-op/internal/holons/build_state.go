package holons

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type persistedBuildState struct {
	Hardened bool `json:"hardened"`
}

func persistBuildMetadata(manifest *LoadedManifest, ctx BuildContext) error {
	if manifest == nil {
		return nil
	}
	if shouldWriteHolonJSON(manifest) {
		if err := writeHolonJSON(manifest, ctx); err != nil {
			return err
		}
	} else if err := updateArtifactHolonJSON(manifest, ctx); err != nil {
		return err
	}
	return writeBuildState(manifest, ctx)
}

func writeBuildState(manifest *LoadedManifest, ctx BuildContext) error {
	path := buildStatePath(manifest)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(persistedBuildState{Hardened: ctx.Hardened}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func readBuildState(manifest *LoadedManifest) (*persistedBuildState, error) {
	path := buildStatePath(manifest)
	if path == "" {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state persistedBuildState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse %s: %w", workspaceRelativePath(path), err)
	}
	return &state, nil
}

func buildStatePath(manifest *LoadedManifest) string {
	if manifest == nil {
		return ""
	}
	return filepath.Join(manifest.OpRoot(), "build", ".build-context.json")
}

func existingBuildHardened(manifest *LoadedManifest, ctx BuildContext) (bool, bool) {
	if state, err := readBuildState(manifest); err == nil {
		return state.Hardened, true
	}

	for _, pkgDir := range []string{
		manifest.HolonPackageDir(),
		sharedHolonPackageDir(manifest),
	} {
		if pkgDir == "" {
			continue
		}
		pkg, err := readHolonPackageJSON(pkgDir)
		if err == nil {
			return pkg.Hardened, true
		}
	}

	artifactPath := manifest.ArtifactPath(ctx)
	if artifactPath != "" {
		if info, err := os.Stat(artifactPath); err == nil && info.IsDir() {
			if pkg, readErr := readHolonPackageJSON(artifactPath); readErr == nil {
				return pkg.Hardened, true
			}
		}
	}

	return false, false
}

func buildContextChangeCleanReason(manifest *LoadedManifest, ctx BuildContext) string {
	if manifest == nil {
		return ""
	}

	hardened, ok := existingBuildHardened(manifest, ctx)
	if ok {
		if hardened == ctx.Hardened {
			return ""
		}
		return fmt.Sprintf(
			"build state changed: previous artifact was %s, cleaning before %s rebuild for sandbox-safe packaging",
			describeHardenedState(hardened),
			describeHardenedState(ctx.Hardened),
		)
	}

	if ctx.Hardened && hasLocalBuildOutputs(manifest, ctx) {
		return "build state changed: existing artifact predates hardened metadata, cleaning before hardened rebuild for sandbox-safe packaging"
	}

	return ""
}

func hasLocalBuildOutputs(manifest *LoadedManifest, ctx BuildContext) bool {
	if manifest == nil {
		return false
	}
	for _, path := range []string{
		buildStatePath(manifest),
		manifest.ArtifactPath(ctx),
		manifest.BinaryPath(),
		manifest.HolonPackageDir(),
	} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func describeHardenedState(hardened bool) string {
	if hardened {
		return "hardened"
	}
	return "non-hardened"
}
