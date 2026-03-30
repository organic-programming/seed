package api_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"

	"google.golang.org/protobuf/encoding/protojson"
)

func TestRunCLIVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := api.RunCLIWithVersion([]string{"version"}, "0.1.0-test", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLIWithVersion returned %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "op 0.1.0-test" {
		t.Fatalf("version output = %q, want %q", got, "op 0.1.0-test")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLIHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := api.RunCLI([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLI returned %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Organic Programming CLI") {
		t.Fatalf("help output missing usage banner: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// TestRunCLIListJSON, TestRunCLICheckJSON, TestRunCLIInspectJSON removed as discovery is no longer proto-based.
func TestRunCLIModInitJSON(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := api.RunCLI([]string{"--format", "json", "mod", "init", "sample/alpha"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLI returned %d, want 0; stderr=%s", code, stderr.String())
	}

	var resp opv1.ModInitResponse
	if err := protojson.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("invalid mod init json: %v\noutput=%s", err, stdout.String())
	}
	if got := filepath.Base(resp.GetModFile()); got != "holon.mod" {
		t.Fatalf("mod file basename = %q, want %q", got, "holon.mod")
	}
	if got := resp.GetHolonPath(); got != "sample/alpha" {
		t.Fatalf("holon_path = %q, want %q", got, "sample/alpha")
	}
}
