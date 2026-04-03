package build_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildOpBootstrap(t *testing.T) {
	// Configure the sandbox respecting Ader isolation (ADER_RUN_ARTIFACTS)
	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")

	// Phase 1: Native 'go build' inside our sandbox (Generation 1)
	gen1Bin := filepath.Join(opBin, "op-gen1")
	t.Logf("Phase 1: Native build (Generation 1) to %s", gen1Bin)
	cmdGen1 := exec.Command("go", "build", "-o", gen1Bin, "../../../../../holons/grace-op/cmd/op")
	if out, err := cmdGen1.CombinedOutput(); err != nil {
		t.Fatalf("Phase 1 failed (native go build): %v\nOutput: %s", err, string(out))
	}

	// Prepare common isolated environment (OPPATH/OPBIN)
	envVars := append(os.Environ(), "OPPATH="+opPath, "OPBIN="+opBin)

	// Phase 2: Execute Generation 1 binary to compile 'op' (Generation 2)
	// (equivalent to running the compiled 'op' to build 'op')
	t.Log("Phase 2: Generation 1 builds Generation 2 (op build op --install)")
	cmdGen2 := exec.Command(gen1Bin, "build", "op", "--install", "--symlink", "--root", "../../../../..")
	cmdGen2.Env = envVars
	if out, err := cmdGen2.CombinedOutput(); err != nil {
		t.Fatalf("Phase 2 failed (Gen1 builds Gen2): %v\nOutput: %s", err, string(out))
	}
	
	// The symlinked binary built by op is named "op"
	gen2Bin := filepath.Join(opBin, "op")
	if stat, err := os.Stat(gen2Bin); os.IsNotExist(err) || stat.Size() == 0 {
		t.Fatalf("Phase 2 did not produce the expected binary %s", gen2Bin)
	}

	// Phase 3: The Gen2 binary compiles 'op' in turn (Generation 3)
	// This is the ultimate self-referential case!
	t.Log("Phase 3: Generation 2 builds Generation 3 (op build op)")
	cmdGen3 := exec.Command(gen2Bin, "build", "op", "--install", "--symlink", "--root", "../../../../..")
	cmdGen3.Env = envVars
	if out, err := cmdGen3.CombinedOutput(); err != nil {
		t.Fatalf("Phase 3 failed (Gen2 builds Gen3): %v\nOutput: %s", err, string(out))
	}

	t.Log("The complete self-referential bootstrap cycle succeeded.")
}
