package sdkprebuilts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBuildScriptReportsBothOutputStreams(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			runtimeHome := t.TempDir()
			t.Setenv("OPPATH", runtimeHome)
			repoRoot := t.TempDir()
			toolchain := testSeedToolchainYAML(t, "zig", testTarget, "32.0", seedSharedProtoc(t, runtimeHome, "32.0", testTarget))
			if err := os.WriteFile(filepath.Join(repoRoot, "seed-toolchain.yaml"), []byte(toolchain), 0o644); err != nil {
				t.Fatal(err)
			}

			var output strings.Builder
			for i := 0; i < 50; i++ {
				fmt.Fprintf(&output, "building target %02d\n", i)
			}
			output.WriteString("FAILED: native-library.o\nerror: incompatible native headers")
			warning := "CMake Deprecation Warning: old policy\n"
			script := filepath.Join(repoRoot, "build.sh")
			body := "#!/usr/bin/env bash\nprintf '%s' " + shellQuote(output.String()) + "\nprintf '%s' " + shellQuote(warning) + " >&2\nexit 1\n"
			if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			opts := BuildOptions{}
			if stream {
				opts.Stdout = &stdout
				opts.Stderr = &stderr
			}
			err := runBuildScript(context.Background(), script, "zig", testTarget, "0.1.0", opts, repoRoot)
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("runBuildScript() error = %v, want wrapped exit status 1", err)
			}
			for _, want := range []string{
				"last stdout lines:\n",
				"FAILED: native-library.o\nerror: incompatible native headers",
				"last stderr lines:\n" + strings.TrimSpace(warning),
			} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want %q", err, want)
				}
			}
			if strings.Contains(err.Error(), "building target 00") {
				t.Errorf("error includes old output instead of only its tail: %v", err)
			}
			if stream && (stdout.String() != output.String() || stderr.String() != warning) {
				t.Errorf("streamed output changed: stdout=%q, stderr=%q", &stdout, &stderr)
			}
		})
	}
}
