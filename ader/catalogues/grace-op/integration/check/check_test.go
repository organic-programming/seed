package check

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

var rootPath string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	// Walk back from seed/ader/catalogues/grace-op/integration/check to the root folder.
	rootPath = filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(cwd))))))
}

// TestCheck_01_GoRun ensures compilation and raw checking works via 'go run'.
func TestCheck_01_GoRun(t *testing.T) {
	integration.TeardownHolons(t, rootPath)

	opPath := t.TempDir()
	opBin := filepath.Join(opPath, "bin")
	envVars := append(os.Environ(), "OPPATH="+opPath, "OPBIN="+opBin)

	cmd := exec.Command("go", "run", "./holons/grace-op/cmd/op", "check", "op", "--root", rootPath)
	cmd.Env = envVars
	cmd.Dir = rootPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run check failed: %v\nOutput: %s", err, string(out))
	}
	output := string(out)

	if !strings.Contains(output, "Operation: check") {
		t.Fatalf("stdout missing standard check protocol: %s", output)
	}
}

// TestCheck_02_SelfCheck asserts that the pre-compiled `op` handles self assessment neutrally.
func TestCheck_02_SelfCheck(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)

	cmd := exec.Command(opBin, "check", "op", "--root", rootPath)
	cmd.Env = envVars
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Self check failed: %v\nOutput: %s", err, string(out))
	}

	output := string(out)
	if !strings.Contains(output, "Runner: go-module") {
		t.Fatalf("Self check failed to resolve accurate Runner format: %s", output)
	}
}

// TestCheck_03_Flags validates modifiers and rendering structures for check.
func TestCheck_03_Flags(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	testHolon := "gabriel-greeting-go"

	t.Run("FormatJson", func(t *testing.T) {
		cmd := exec.Command(opBin, "check", testHolon, "--format", "json", "--root", rootPath)
		cmd.Env = envVars
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("JSON format check failed: %v\nOutput: %s", err, string(out))
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("Could not parse JSON verification payload: %v\nOutput: %s", err, string(out))
		}

		if payload["operation"] != "check" {
			t.Fatalf("operation = %v, expected check", payload["operation"])
		}
	})

	t.Run("CwdTargeting", func(t *testing.T) {
		holonDir := filepath.Join(rootPath, "examples/hello-world", testHolon)
		cmd := exec.Command(opBin, "check", "--cwd")
		cmd.Env = envVars
		cmd.Dir = holonDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("--cwd check failed: %v\nOutput: %s", err, string(out))
		}
		
		output := string(out)
		if !strings.Contains(output, "Operation: check") {
			t.Fatalf("Failed to implicitly identify directory context payload: %s", output)
		}
	})
}

// TestCheck_04_Matrix systematically verifies cross-language manifest resolutions universally.
func TestCheck_04_Matrix(t *testing.T) {
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
			cmd := exec.Command(opBin, "check", ex, "--root", rootPath)
			cmd.Env = envVars
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("Check failed for matrix candidate %s: %v\nOutput: %s", ex, err, string(out))
			} else if !strings.Contains(string(out), "Operation: check") {
				t.Fatalf("Missing Operation: check payload for %s: %s", ex, string(out))
			}
		})
	}
}

// TestCheck_05_Composite verifies graph resolution for non-native compositional structures.
func TestCheck_05_Composite(t *testing.T) {
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)

	cmd := exec.Command(opBin, "check", "gabriel-greeting-app-swiftui", "--root", rootPath)
	cmd.Env = envVars
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Composite check failed: %v\nOutput: %s", err, string(out))
	}

	if !strings.Contains(string(out), "Operation: check") {
		t.Fatalf("Failed to parse SwiftUI composite graph: %s", string(out))
	}
}
