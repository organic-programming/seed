package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkgrpc "github.com/organic-programming/go-holons/pkg/grpcclient"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
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

	envVars := withEnv(os.Environ(), "OPPATH", opPath)
	envVars = withEnv(envVars, "OPBIN", opBin)
	envVars = withEnv(envVars, "PATH", FilterInstalledHolonsPath(os.Getenv("PATH")))

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

// SetupStdioOPClient launches the OP binary in stdio gRPC mode and returns a typed client.
func SetupStdioOPClient(t *testing.T, rootDir, opBin string, envVars []string) (opv1.OPServiceClient, func()) {
	t.Helper()

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		t.Fatalf("Failed to resolve root %s: %v", rootDir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.Command(opBin, "serve", "--listen", "stdio://")
	cmd.Dir = absRoot
	cmd.Env = withEnv(envVars, "OPROOT", absRoot)

	conn, startedCmd, err := sdkgrpc.DialStdioCommand(ctx, cmd)
	if err != nil {
		cancel()
		t.Fatalf("Failed to start stdio RPC client for %s: %v", opBin, err)
	}

	cleanup := func() {
		_ = conn.Close()
		if startedCmd != nil && startedCmd.Process != nil {
			_ = startedCmd.Process.Kill()
			_ = startedCmd.Wait()
		}
		cancel()
	}

	return opv1.NewOPServiceClient(conn), cleanup
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

// FilterInstalledHolonsPath removes previously installed holon bins from PATH.
func FilterInstalledHolonsPath(pathValue string) string {
	if strings.TrimSpace(pathValue) == "" {
		return pathValue
	}

	entries := strings.Split(pathValue, string(os.PathListSeparator))
	filtered := make([]string, 0, len(entries))
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		cleaned := filepath.Clean(trimmed)
		lower := strings.ToLower(cleaned)
		if strings.Contains(lower, strings.ToLower(string(filepath.Separator)+".op"+string(filepath.Separator)+"bin")) {
			continue
		}
		if strings.HasSuffix(lower, strings.ToLower(filepath.Join(".op", "bin"))) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(filtered, string(os.PathListSeparator))
}

func withEnv(envVars []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(envVars)+1)
	for _, entry := range envVars {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return append(out, prefix+value)
}
