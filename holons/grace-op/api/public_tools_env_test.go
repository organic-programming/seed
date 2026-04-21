package api_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)


func TestEnvInitializesDirectoriesAndShell(t *testing.T) {
	root := t.TempDir()
	oppath := filepath.Join(root, ".op")
	opbin := filepath.Join(oppath, "bin")
	t.Setenv("OPPATH", oppath)
	t.Setenv("OPBIN", opbin)

	resp, err := api.Env(&opv1.EnvRequest{Init: true, Shell: true})
	if err != nil {
		t.Fatalf("Env error = %v", err)
	}
	if resp.GetOppath() != oppath {
		t.Fatalf("OPPATH = %q, want %q", resp.GetOppath(), oppath)
	}
	if resp.GetOpbin() != opbin {
		t.Fatalf("OPBIN = %q, want %q", resp.GetOpbin(), opbin)
	}
	if _, err := os.Stat(opbin); err != nil {
		t.Fatalf("OPBIN directory missing: %v", err)
	}
	if !strings.Contains(resp.GetShell(), "export OPPATH") {
		t.Fatalf("shell snippet = %q, want export OPPATH", resp.GetShell())
	}
}
