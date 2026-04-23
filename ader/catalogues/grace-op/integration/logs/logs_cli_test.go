//go:build e2e

package logs_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

// TestLogs_CLI_NoInstance verifies the CLI fails gracefully when no
// running instance matches the slug.
func TestLogs_CLI_NoInstance(t *testing.T) {
	sb := integration.NewSandbox(t)
	res := sb.RunOP(t, "logs", "no-such-slug", "--follow=false")
	integration.RequireFailure(t, res)
	if !strings.Contains(res.Stderr, "no running instance") {
		t.Fatalf("expected not-found message; stderr:\n%s", res.Stderr)
	}
}

// TestLogs_CLI_Help verifies the command exposes expected flags.
func TestLogs_CLI_Help(t *testing.T) {
	sb := integration.NewSandbox(t)
	res := sb.RunOP(t, "logs", "--help")
	integration.RequireSuccess(t, res)
	for _, flag := range []string{"--since", "--level", "--session", "--method", "--follow", "--json", "--chain-origin"} {
		if !strings.Contains(res.Stdout, flag) {
			t.Errorf("missing flag %s in help; got:\n%s", flag, res.Stdout)
		}
	}
}
