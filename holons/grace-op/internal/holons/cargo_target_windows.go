package holons

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	openv "github.com/organic-programming/grace-op/internal/env"
)

func cargoTargetDir(manifest *LoadedManifest) string {
	// Native CMake subprojects (notably Vulkan's shader generator) exceed
	// MAX_PATH even with Ninja under a normal checkout. Hash the absolute source
	// path, not the slug, so independent worktrees and Ader snapshots never use
	// each other's build outputs. Cargo keeps Debug/Release below this root.
	absolute, err := filepath.Abs(manifest.Dir)
	if err != nil {
		absolute = manifest.Dir
	}
	key := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(absolute))))
	// Base32 retains all 64 identity bits with thirteen filename characters.
	// Those three saved characters also matter to MSVC's nested manifest path.
	name := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key[:8]))
	return filepath.Join(openv.OPPATH(), "b", name)
}

func cargoTargetEnv(manifest *LoadedManifest) []string {
	if manifest == nil || manifest.Manifest.Build.Runner != RunnerCargo || hasCMakeProject(manifest) {
		return nil
	}
	root := cargoTargetDir(manifest)
	return []string{"CARGO_TARGET_DIR=" + root, "OP_CARGO_TARGET_DIR=" + root}
}

func cargoHookArgs(manifest *LoadedManifest, cwd string, args []string) []string {
	if manifest.Manifest.Build.Runner != RunnerCargo || len(args) == 0 {
		return args
	}
	if name := strings.ToLower(filepath.Base(args[0])); name != "cargo" && name != "cargo.exe" {
		return args
	}
	result := append([]string(nil), args...)
	canonical := filepath.Join(manifest.OpRoot(), "build", "cargo")
	for i := 1; i < len(result); i++ {
		value, equals := strings.CutPrefix(result[i], "--target-dir=")
		if result[i] == "--target-dir" && i+1 < len(result) {
			i++
			value, equals = result[i], false
		} else if !equals {
			continue
		}
		resolved := value
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(cwd, value)
		}
		if strings.EqualFold(filepath.Clean(resolved), canonical) {
			result[i] = cargoTargetDir(manifest)
			if equals {
				result[i] = "--target-dir=" + result[i]
			}
		}
	}
	return result
}

func prepareCargoTargetDir(manifest *LoadedManifest) error {
	root := cargoTargetDir(manifest)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	marker := filepath.Join(root, ".op-source")
	owner, err := os.ReadFile(marker)
	if err == nil && !strings.EqualFold(string(owner), filepath.Clean(manifest.Dir)) {
		return fmt.Errorf("Cargo target belongs to another source: %s", root)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(marker, []byte(filepath.Clean(manifest.Dir)), 0o600)
}

func cleanCargoTargetDir(manifest *LoadedManifest) error {
	root := cargoTargetDir(manifest)
	owner, err := os.ReadFile(filepath.Join(root, ".op-source"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(string(owner), filepath.Clean(manifest.Dir)) {
		return fmt.Errorf("refusing to clean Cargo target owned by another source: %s", root)
	}
	return os.RemoveAll(root)
}
