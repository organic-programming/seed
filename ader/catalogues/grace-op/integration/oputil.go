package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// SetupIsolatedOP creates an isolated OPPATH and builds the 'op' orchestrator.
// It returns the environment variables to inject and the path to the installed 'op' binary.
func SetupIsolatedOP(t *testing.T, rootDir string) ([]string, string) {
	t.Helper()
	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")

	// Phase 1: Native 'go build'
	gen1Bin := filepath.Join(opBin, "op-gen1")
	cmdGen1 := exec.Command("go", "build", "-o", gen1Bin, filepath.Join(rootDir, "holons/grace-op/cmd/op"))
	if out, err := cmdGen1.CombinedOutput(); err != nil {
		t.Fatalf("Failed native go build: %v\nOutput: %s", err, string(out))
	}

	envVars := append(os.Environ(), "OPPATH="+opPath, "OPBIN="+opBin)

	// Phase 2: Generation 1 builds Generation 2 (the final op binary)
	cmdGen2 := exec.Command(gen1Bin, "build", "op", "--install", "--symlink", "--root", rootDir)
	cmdGen2.Env = envVars
	if out, err := cmdGen2.CombinedOutput(); err != nil {
		t.Fatalf("Failed OP bootstrap build: %v\nOutput: %s", err, string(out))
	}

	gen2Bin := filepath.Join(opBin, "op")
	if stat, err := os.Stat(gen2Bin); os.IsNotExist(err) || stat.Size() == 0 {
		t.Fatalf("Bootstrap did not produce the expected binary %s", gen2Bin)
	}

	return envVars, gen2Bin
}

// TeardownHolons vigorously wipes the .op/build specific cache directories across all examples
// to natively guarantee an absolute zero-state environment for the test framework.
func TeardownHolons(t *testing.T, rootDir string) {
	t.Helper()
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
}
