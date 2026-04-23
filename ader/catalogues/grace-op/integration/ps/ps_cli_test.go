//go:build e2e

package ps_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

// TestPs_CLI_Empty verifies `op ps` on a fresh sandbox emits the
// "No running instances." marker and exits 0.
func TestPs_CLI_Empty(t *testing.T) {
	sb := integration.NewSandbox(t)
	res := sb.RunOP(t, "ps")
	integration.RequireSuccess(t, res)
	if !strings.Contains(res.Stdout, "No running instances") {
		t.Fatalf("expected empty-ps marker in stdout; got:\n%s", res.Stdout)
	}
}

// TestPs_CLI_JSONEmpty verifies the --json form emits valid JSON
// (empty array) on an empty registry.
func TestPs_CLI_JSONEmpty(t *testing.T) {
	sb := integration.NewSandbox(t)
	res := sb.RunOP(t, "ps", "--json")
	integration.RequireSuccess(t, res)
	trimmed := strings.TrimSpace(res.Stdout)
	if trimmed != "[]" && trimmed != "null" {
		t.Fatalf("expected [] or null for empty JSON ps; got:\n%s", res.Stdout)
	}
}
