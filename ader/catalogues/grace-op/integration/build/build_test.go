package build_test

import (
	"os/exec"
	"testing"
	
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

const rootPath = "../../../../.."

func TestBootstrapOP(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)

	t.Log("Phase 3: Generation 2 builds Generation 3 (op build op)")
	cmdGen3 := exec.Command(opBin, "build", "op", "--install", "--symlink", "--root", rootPath)
	cmdGen3.Env = envVars
	if out, err := cmdGen3.CombinedOutput(); err != nil {
		t.Fatalf("Phase 3 self-referential build failed: %v\nOutput: %s", err, string(out))
	}
}

func TestBuildMatrix(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)

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
			cmd := exec.Command(opBin, "build", ex, "--root", rootPath)
			cmd.Env = envVars
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("Failed to build %s: %v\nOutput: %s", ex, err, string(out))
			}
		})
	}
}

func TestBuildComposite(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)

	t.Log("Building composite app gabriel-greeting-app-swiftui from absolute zero state...")
	cmd := exec.Command(opBin, "build", "gabriel-greeting-app-swiftui", "--root", rootPath)
	cmd.Env = envVars
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build composite SwiftUI app: %v\nOutput: %s", err, string(out))
	}
}
