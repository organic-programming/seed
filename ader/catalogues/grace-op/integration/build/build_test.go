package build_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"runtime"
	"regexp"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

const rootPath = "../../../../.."

// TestBuild_01_GoBuild evaluates the baseline Go compilation of the CLI source.
func TestBuild_01_GoBuild(t *testing.T) {
	integration.TeardownHolons(t, rootPath)

	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")

	gen1Bin := filepath.Join(opBin, "op-gen1")
	t.Logf("Level 1: Native build to %s", gen1Bin)
	cmdGen1 := exec.Command("go", "build", "-o", gen1Bin, filepath.Join(rootPath, "holons/grace-op/cmd/op"))
	if out, err := cmdGen1.CombinedOutput(); err != nil {
		t.Fatalf("Level 1 failed (native go build): %v\nOutput: %s", err, string(out))
	}
}

// TestBuild_02_GoRun evaluates that `go run` can execute the CLI source to build op itself.
func TestBuild_02_GoRun(t *testing.T) {
	integration.TeardownHolons(t, rootPath)

	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")
	envVars := append(os.Environ(), "OPPATH="+opPath, "OPBIN="+opBin)

	t.Log("Level 2: `go run` builds op (op build op --install)")
	cmd := exec.Command("go", "run", filepath.Join(rootPath, "holons/grace-op/cmd/op"), "build", "op", "--install", "--symlink", "--root", rootPath)
	cmd.Env = envVars
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Level 2 failed (go run): %v\nOutput: %s", err, string(out))
	}

	symlinkedBin := filepath.Join(opBin, "op")
	if stat, err := os.Stat(symlinkedBin); os.IsNotExist(err) || stat.Size() == 0 {
		t.Fatalf("Level 2 did not produce the expected symlinked binary %s", symlinkedBin)
	}
}

// TestBuild_03_OpBuildSelf evaluates the ultimate self-referential scenario where the binary builds itself.
func TestBuild_03_OpBuildSelf(t *testing.T) {
	integration.TeardownHolons(t, rootPath)

	// We utilize the Setup helper to obtain a clean OP executable to test self-compilation
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	
	t.Log("Level 3: Generation 2 builds Generation 3 (op build op)")
	cmd := exec.Command(opBin, "build", "op", "--install", "--symlink", "--root", rootPath)
	cmd.Env = envVars
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Level 3 self-referential build failed: %v\nOutput: %s", err, string(out))
	}
}

// TestBuild_04_Flags evaluates the functionality of configuration CLI flags.
func TestBuild_04_Flags(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	opBinDir := filepath.Dir(opBin)
	testHolon := "gabriel-greeting-go"

	t.Run("DryRun", func(t *testing.T) {
		cmd := exec.Command(opBin, "build", testHolon, "--dry-run", "--root", rootPath)
		cmd.Env = envVars
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Dry run failed: %v\nOutput: %s", err, string(out))
		}
		// Dry run should not output a usable binary in OPBIN since we didn't install, but even in its local build folder it should be empty
		// We'll just verify the command succeeded cleanly
		if !strings.Contains(string(out), "gabriel-greeting-go") {
			t.Errorf("Dry run output did not display the expected holon plan")
		}
	})

	t.Run("Install", func(t *testing.T) {
		cmd := exec.Command(opBin, "build", testHolon, "--install", "--root", rootPath)
		cmd.Env = envVars
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Install flag failed: %v\nOutput: %s", err, string(out))
		}
	})

	t.Run("Symlink", func(t *testing.T) {
		cmd := exec.Command(opBin, "build", testHolon, "--install", "--symlink", "--root", rootPath)
		cmd.Env = envVars
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Symlink flag failed: %v\nOutput: %s", err, string(out))
		}
		
		symlinkTarget := filepath.Join(opBinDir, testHolon)
		if stat, err := os.Stat(symlinkTarget); os.IsNotExist(err) || stat.Size() == 0 {
			t.Fatalf("Symlink flag did not produce the expected symlink binary %s", symlinkTarget)
		}
	})

	t.Run("Clean", func(t *testing.T) {
		cmd := exec.Command(opBin, "build", testHolon, "--clean", "--root", rootPath)
		cmd.Env = envVars
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Clean flag failed: %v\nOutput: %s", err, string(out))
		}
		
		// The local holon .op cache was cleaned and then rebuilt seamlessly
		localOpPath := filepath.Join(rootPath, "examples", "hello-world", testHolon, ".op", "build", testHolon+".holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, testHolon)
		if stat, err := os.Stat(localOpPath); os.IsNotExist(err) || stat.Size() == 0 {
			t.Fatalf("Clean flag prevented the expected local artifact creation at %s", localOpPath)
		}
	})
	
	t.Run("Quiet", func(t *testing.T) {
		cmd := exec.Command(opBin, "build", testHolon, "--quiet", "--root", rootPath)
		cmd.Env = envVars
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Quiet flag failed: %v\nOutput: %s", err, string(out))
		}
	})
}

// TestBuild_05_SymlinkOverwrite ensures repeated builds properly overwrite installed artifacts natively.
func TestBuild_05_SymlinkOverwrite(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	opBinDir := filepath.Dir(opBin)
	testHolon := "gabriel-greeting-go"

	// Action 1: Initial build
	t.Log("Action 1: Initial install/symlink")
	cmd1 := exec.Command(opBin, "build", testHolon, "--install", "--symlink", "--root", rootPath)
	cmd1.Env = envVars
	if out, err := cmd1.CombinedOutput(); err != nil {
		t.Fatalf("Initial build failed: %v\nOutput: %s", err, string(out))
	}

	// Read initial version from the built binary
	installedBin := filepath.Join(opBinDir, testHolon)
	outVersion1, err := exec.Command(installedBin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to execute installed binary: %v", err)
	}
	t.Logf("Initial version output: %s", strings.TrimSpace(string(outVersion1)))

	// Mutate the local holon.proto
	protoPath := filepath.Join(rootPath, "examples/hello-world", testHolon, "api/v1/holon.proto")
	content, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("Failed to read holon.proto: %v", err)
	}

	// Regex to replace version to 9.9.99
	re := regexp.MustCompile(`version:\s*".*?"`)
	mutatedContent := re.ReplaceAllString(string(content), `version: "9.9.99"`)

	// Register defer to strictly restore the original file contents cleanly
	defer os.WriteFile(protoPath, content, 0644)

	if err := os.WriteFile(protoPath, []byte(mutatedContent), 0644); err != nil {
		t.Fatalf("Failed to write mutated holon.proto: %v", err)
	}

	// Action 2: Re-build and overwrite
	t.Log("Action 2: Rebuild with mutated version")
	cmd2 := exec.Command(opBin, "build", testHolon, "--install", "--symlink", "--root", rootPath)
	cmd2.Env = envVars
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Fatalf("Overwrite build failed: %v\nOutput: %s", err, string(out))
	}

	// Assertion 2: Verify the symlink now executes the 9.9.99 version
	outVersion2, err := exec.Command(installedBin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to execute overwritten binary: %v", err)
	}

	finalVersion := strings.TrimSpace(string(outVersion2))
	t.Logf("Final version output: %s", finalVersion)

	if !strings.Contains(finalVersion, "9.9.100") {
		t.Fatalf("Regression! Symlink version not updated.\nExpected: 9.9.100 (auto-incremented from 9.9.99 due to dirty tree)\nGot: %s", finalVersion)
	}
}

// TestBuild_06_Matrix evaluates op build capability comprehensively across the 12 example languages.
func TestBuild_06_Matrix(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	opBinDir := filepath.Dir(opBin)

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
			t.Logf("Mutating layer version and Building %s...", ex)

			// Force local version mutation to 8.8.88 to trigger compiling backend parity assertion
			protoPath := filepath.Join(rootPath, "examples/hello-world", ex, "api/v1/holon.proto")
			content, err := os.ReadFile(protoPath)
			if err != nil {
				t.Fatalf("Failed to read holon.proto for %s: %v", ex, err)
			}

			re := regexp.MustCompile(`version:\s*".*?"`)
			mutatedContent := re.ReplaceAllString(string(content), `version: "8.8.88"`)
			
			// Register defer to strictly restore the workspace 
			defer os.WriteFile(protoPath, content, 0644)

			if err := os.WriteFile(protoPath, []byte(mutatedContent), 0644); err != nil {
				t.Fatalf("Failed to explicitly mutate proto for %s: %v", ex, err)
			}

			// Compile via op build with install flag to access the binary universally
			cmd := exec.Command(opBin, "build", ex, "--install", "--symlink", "--root", rootPath)
			cmd.Env = envVars
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("Failed to build %s: %v\nOutput: %s", ex, err, string(out))
			}

			// Assertion: execute output binary and rigorously expect the bump
			installedBin := filepath.Join(opBinDir, ex)
			outVersion, err := exec.Command(installedBin, "version").CombinedOutput()
			if err != nil {
				t.Fatalf("Failed to natively execute %s binary: %v", ex, err)
			}

			finalVersion := strings.TrimSpace(string(outVersion))
			t.Logf("[%s] Final runtime output: %s", ex, finalVersion)

			if !strings.Contains(finalVersion, "8.8.89") {
				t.Fatalf("Regression! Auto-increment backend failed on %s runtime wrapper.\nExpected contain: 8.8.89\nGot: %s", ex, finalVersion)
			}
		})
	}
}

// TestBuild_07_Composite evaluates the swiftui app composite capability without caching bias.
func TestBuild_07_Composite(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)

	t.Log("Building composite app gabriel-greeting-app-swiftui from absolute zero state...")
	cmd := exec.Command(opBin, "build", "gabriel-greeting-app-swiftui", "--root", rootPath)
	cmd.Env = envVars
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build composite SwiftUI app: %v\nOutput: %s", err, string(out))
	}
}
