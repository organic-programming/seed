//go:build !windows

package holons

import (
	"os"
	"path/filepath"
)

func cargoTargetDir(manifest *LoadedManifest) string {
	return filepath.Join(manifest.OpRoot(), "build", "cargo")
}
func cargoTargetEnv(*LoadedManifest) []string                           { return nil }
func cargoHookArgs(_ *LoadedManifest, _ string, args []string) []string { return args }
func prepareCargoTargetDir(manifest *LoadedManifest) error {
	return os.MkdirAll(cargoTargetDir(manifest), 0o755)
}
func cleanCargoTargetDir(*LoadedManifest) error { return nil }
