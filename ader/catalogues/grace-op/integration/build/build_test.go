package build_test

import (
	"os/exec"
	"testing"
	
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

const rootPath = "../../../../.."

func TestBootstrapOP(t *testing.T) {
	integration.TeardownHolons(t, rootPath)

	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")
	envVars := append(os.Environ(), "OPPATH="+opPath, "OPBIN="+opBin)

	// Level 1: Native 'go build' inside our sandbox (Generation 1)
	gen1Bin := filepath.Join(opBin, "op-gen1")
	t.Logf("Phase 1(Level 1): Native build (Generation 1) to %s", gen1Bin)
	cmdGen1 := exec.Command("go", "build", "-o", gen1Bin, filepath.Join(rootPath, "holons/grace-op/cmd/op"))
	if out, err := cmdGen1.CombinedOutput(); err != nil {
		t.Fatalf("Phase 1 failed (native go build): %v\nOutput: %s", err, string(out))
	}

	// Level 2: Execute Generation 1 binary to compile 'op' (Generation 2)
	t.Log("Phase 2(Level 2): Generation 1 builds Generation 2 (op build op --install)")
	cmdGen2 := exec.Command(gen1Bin, "build", "op", "--install", "--symlink", "--root", rootPath)
	cmdGen2.Env = envVars
	if out, err := cmdGen2.CombinedOutput(); err != nil {
		t.Fatalf("Phase 2 failed (Gen1 builds Gen2): %v\nOutput: %s", err, string(out))
	}

	gen2Bin := filepath.Join(opBin, "op")
	if stat, err := os.Stat(gen2Bin); os.IsNotExist(err) || stat.Size() == 0 {
		t.Fatalf("Phase 2 did not produce the expected binary %s", gen2Bin)
	}

	// Level 3: The Gen2 binary compiles 'op' in turn (Generation 3)
	t.Log("Phase 3(Level 3): Generation 2 builds Generation 3 (op build op)")
	cmdGen3 := exec.Command(gen2Bin, "build", "op", "--install", "--symlink", "--root", rootPath)
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
