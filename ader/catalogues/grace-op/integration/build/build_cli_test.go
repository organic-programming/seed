//go:build e2e

package build_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

// TestBuild_01_GoBuild evaluates the baseline Go compilation of the CLI source.
func TestBuild_01_GoBuild(t *testing.T) {
	rootPath := absoluteRootPath(t)
	integration.TeardownHolons(t, rootPath)

	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")

	gen1Bin := filepath.Join(opBin, "op-gen1")
	t.Logf("Level 1: Native build to %s", gen1Bin)
	cmdGen1 := exec.Command("go", "build", "-o", gen1Bin, filepath.Join(rootPath, "holons/grace-op/cmd/op"))
	cmdGen1.Dir = rootPath
	if out, err := cmdGen1.CombinedOutput(); err != nil {
		t.Fatalf("Level 1 failed (native go build): %v\nOutput: %s", err, string(out))
	}
}

// TestBuild_02_GoRun evaluates that `go run` can execute the CLI source to build op itself.
func TestBuild_02_GoRun(t *testing.T) {
	rootPath := absoluteRootPath(t)
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
	cmd.Dir = rootPath
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
	rootPath := absoluteRootPath(t)
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
	rootPath := absoluteRootPath(t)
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
	rootPath := absoluteRootPath(t)
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

	if !strings.Contains(finalVersion, "9.9.99") {
		t.Fatalf("Regression! Symlink did not pick up the mutated version.\nExpected: 9.9.99 (no --bump, proto version is authoritative)\nGot: %s", finalVersion)
	}
}

// TestBuild_06_Matrix evaluates op build comprehensively across the example language matrix.
func TestBuild_06_Matrix(t *testing.T) {
	rootPath := absoluteRootPath(t)
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	opBinDir := filepath.Dir(opBin)

	examples := integration.AvailableHelloWorldSlugs(t, false)

	for _, ex := range examples {
		t.Run(ex, func(t *testing.T) {
			withMutatedHolonVersion(t, ex, "8.8.88", func() {
				t.Logf("Building %s through the CLI...", ex)

				cmd := exec.Command(opBin, "build", ex, "--install", "--symlink", "--root", rootPath)
				cmd.Env = envVars
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("Failed to build %s: %v\nOutput: %s", ex, err, string(out))
				}

				isolatedExeDir := filepath.Join(opBinDir, ex+"_isolated")
				binaryPath := copyBinaryBundle(t, ex, isolatedExeDir)
				finalVersion := runInstalledBinary(t, binaryPath, "version")
				t.Logf("[%s] Final runtime output: %s", ex, finalVersion)

				if runtime.GOOS == "darwin" && (ex == "gabriel-greeting-c" || ex == "gabriel-greeting-cpp") {
					assertBundledDarwinDeps(t, binaryPath)
				}

				if !strings.Contains(finalVersion, "8.8.88") {
					t.Fatalf("Regression! Template substitution failed on %s.\nExpected contain: 8.8.88 (no --bump, proto version is authoritative)\nGot: %s", ex, finalVersion)
				}
			})
		})
	}
}

// TestBuild_07_BumpFlag verifies that --bump increments the patch component of
// identity.version in the holon.proto and that the built binary reports the
// new version, while a build without --bump leaves the proto untouched.
func TestBuild_07_BumpFlag(t *testing.T) {
	rootPath := absoluteRootPath(t)
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	testHolon := "gabriel-greeting-go"

	withMutatedHolonVersion(t, testHolon, "7.7.77", func() {
		protoPath := filepath.Join(rootPath, "examples/hello-world", testHolon, "api/v1/holon.proto")
		versionRe := regexp.MustCompile(`version:\s*"([^"]+)"`)

		readProtoVersion := func() string {
			content, err := os.ReadFile(protoPath)
			if err != nil {
				t.Fatalf("read holon.proto: %v", err)
			}
			match := versionRe.FindStringSubmatch(string(content))
			if len(match) != 2 {
				t.Fatalf("could not find version in holon.proto")
			}
			return match[1]
		}

		// Build without --bump: proto version must stay 7.7.77.
		t.Log("Action 1: build without --bump — proto must not mutate")
		cmd1 := exec.Command(opBin, "build", testHolon, "--install", "--symlink", "--root", rootPath)
		cmd1.Env = envVars
		if out, err := cmd1.CombinedOutput(); err != nil {
			t.Fatalf("build without --bump failed: %v\nOutput: %s", err, string(out))
		}
		if got := readProtoVersion(); got != "7.7.77" {
			t.Fatalf("Regression! Build without --bump mutated the proto.\nExpected: 7.7.77\nGot: %s", got)
		}

		// Build with --bump: proto version must advance to 7.7.78, and the
		// installed binary must report that same version.
		t.Log("Action 2: build with --bump — patch must advance to 7.7.78")
		cmd2 := exec.Command(opBin, "build", testHolon, "--bump", "--install", "--symlink", "--root", rootPath)
		cmd2.Env = envVars
		if out, err := cmd2.CombinedOutput(); err != nil {
			t.Fatalf("build with --bump failed: %v\nOutput: %s", err, string(out))
		}
		if got := readProtoVersion(); got != "7.7.78" {
			t.Fatalf("Regression! --bump did not advance the patch.\nExpected: 7.7.78\nGot: %s", got)
		}

		opBinDir := filepath.Dir(opBin)
		installedBin := filepath.Join(opBinDir, testHolon)
		runtimeOut, err := exec.Command(installedBin, "version").CombinedOutput()
		if err != nil {
			t.Fatalf("run installed binary: %v", err)
		}
		if !strings.Contains(strings.TrimSpace(string(runtimeOut)), "7.7.78") {
			t.Fatalf("Regression! --bump not reflected in built binary.\nExpected contain: 7.7.78\nGot: %s", string(runtimeOut))
		}
	})
}

// TestBuild_08_BumpDryRun verifies that --bump --dry-run previews the intended
// version in the progress output without mutating the proto on disk.
func TestBuild_08_BumpDryRun(t *testing.T) {
	rootPath := absoluteRootPath(t)
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	testHolon := "gabriel-greeting-go"

	withMutatedHolonVersion(t, testHolon, "6.6.66", func() {
		protoPath := filepath.Join(rootPath, "examples/hello-world", testHolon, "api/v1/holon.proto")

		cmd := exec.Command(opBin, "build", testHolon, "--bump", "--dry-run", "--root", rootPath)
		cmd.Env = envVars
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("dry-run build with --bump failed: %v\nOutput: %s", err, string(out))
		}

		if !strings.Contains(string(out), "would bump version: 6.6.66 → 6.6.67") {
			t.Fatalf("Regression! --dry-run --bump did not emit the preview note.\nGot: %s", string(out))
		}

		content, err := os.ReadFile(protoPath)
		if err != nil {
			t.Fatalf("read holon.proto: %v", err)
		}
		if !strings.Contains(string(content), `version: "6.6.66"`) {
			t.Fatalf("Regression! --dry-run --bump mutated the proto.\nExpected proto still contains version: \"6.6.66\"\nGot: %s", string(content))
		}
	})
}
