package build_test

import (
	"os/exec"
	"testing"
	
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestBuild(t *testing.T) {
	rootDir := "../../../../.."
	
	// Phase 1 and 2: Bootstrap via shared setup
	envVars, opBin := integration.SetupIsolatedOP(t, rootDir)

	// Phase 3: The ultimate self-referential case
	t.Run("SelfReferential", func(t *testing.T) {
		t.Log("Phase 3: Generation 2 builds Generation 3 (op build op)")
		cmdGen3 := exec.Command(opBin, "build", "op", "--install", "--symlink", "--root", rootDir)
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
				cmd := exec.Command(opBin, "build", ex, "--root", rootDir)
				cmd.Env = envVars
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("Failed to build %s: %v\nOutput: %s", ex, err, string(out))
				}
			})
		}
	})

	t.Run("Composite", func(t *testing.T) {
		// As established, the testing framework natively reverts state here
		integration.TeardownHolons(t, rootDir)
		
		t.Log("Building composite app gabriel-greeting-app-swiftui from absolute zero state...")
		cmd := exec.Command(opBin, "build", "gabriel-greeting-app-swiftui", "--root", rootDir)
		cmd.Env = envVars
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Failed to build composite SwiftUI app: %v\nOutput: %s", err, string(out))
		}
	})
}
