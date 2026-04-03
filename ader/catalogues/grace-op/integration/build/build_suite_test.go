package build_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildSuite(t *testing.T) {
	// Configure the sandbox respecting Ader isolation (ADER_RUN_ARTIFACTS)
	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")
	envVars := append(os.Environ(), "OPPATH="+opPath, "OPBIN="+opBin)
	rootDir := "../../../../.."
	gen2Bin := filepath.Join(opBin, "op")

	t.Run("Bootstrap", func(t *testing.T) {
		gen1Bin := filepath.Join(opBin, "op-gen1")
		t.Logf("Phase 1: Native build (Generation 1) to %s", gen1Bin)
		cmdGen1 := exec.Command("go", "build", "-o", gen1Bin, filepath.Join(rootDir, "holons/grace-op/cmd/op"))
		if out, err := cmdGen1.CombinedOutput(); err != nil {
			t.Fatalf("Phase 1 failed (native go build): %v\nOutput: %s", err, string(out))
		}

		t.Log("Phase 2: Generation 1 builds Generation 2 (op build op --install)")
		cmdGen2 := exec.Command(gen1Bin, "build", "op", "--install", "--symlink", "--root", rootDir)
		cmdGen2.Env = envVars
		if out, err := cmdGen2.CombinedOutput(); err != nil {
			t.Fatalf("Phase 2 failed: %v\nOutput: %s", err, string(out))
		}
		
		if stat, err := os.Stat(gen2Bin); os.IsNotExist(err) || stat.Size() == 0 {
			t.Fatalf("Phase 2 did not produce the expected binary %s", gen2Bin)
		}

		t.Log("Phase 3: Generation 2 builds Generation 3 (op build op)")
		cmdGen3 := exec.Command(gen2Bin, "build", "op", "--install", "--symlink", "--root", rootDir)
		cmdGen3.Env = envVars
		if out, err := cmdGen3.CombinedOutput(); err != nil {
			t.Fatalf("Phase 3 failed: %v\nOutput: %s", err, string(out))
		}
	})

	t.Run("Matrix", func(t *testing.T) {
		examples := []string{
			"gabriel-greeting-c",
			"gabriel-greeting-cpp",
			"gabriel-greeting-csharp",
			"gabriel-greeting-dart",
			"gabriel-greeting-go",
			"gabriel-greeting-java",
			"gabriel-greeting-kotlin",
			"gabriel-greeting-node",
			"gabriel-greeting-python",
			"gabriel-greeting-ruby",
			"gabriel-greeting-rust",
			"gabriel-greeting-swift",
		}

		for _, ex := range examples {
			t.Run(ex, func(t *testing.T) {
				t.Logf("Building %s...", ex)
				cmd := exec.Command(gen2Bin, "build", ex, "--root", rootDir)
				cmd.Env = envVars
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("Failed to build %s: %v\nOutput: %s", ex, err, string(out))
				}
			})
		}
	})

	t.Run("Composite", func(t *testing.T) {
		// Teardown an unalterable zero-level state natively by wiping all `.op` directories.
		// As established, the testing framework handles zero-state natively, not `op clean`.
		examplesDir := filepath.Join(rootDir, "examples/hello-world")
		entries, err := os.ReadDir(examplesDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					opDir := filepath.Join(examplesDir, entry.Name(), ".op")
					_ = os.RemoveAll(opDir)
				}
			}
		}

		t.Log("Building composite app gabriel-greeting-app-swiftui from absolute zero state...")
		cmd := exec.Command(gen2Bin, "build", "gabriel-greeting-app-swiftui", "--root", rootDir)
		cmd.Env = envVars
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Failed to build composite SwiftUI app: %v\nOutput: %s", err, string(out))
		}
	})
}
