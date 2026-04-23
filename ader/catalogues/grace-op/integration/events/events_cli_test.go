//go:build e2e

package events_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestEvents_CLI_NoInstance(t *testing.T) {
	sb := integration.NewSandbox(t)
	res := sb.RunOP(t, "events", "no-such-slug", "--follow=false")
	integration.RequireFailure(t, res)
	if !strings.Contains(res.Stderr, "no running instance") {
		t.Fatalf("expected not-found message; stderr:\n%s", res.Stderr)
	}
}

func TestEvents_CLI_Help(t *testing.T) {
	sb := integration.NewSandbox(t)
	res := sb.RunOP(t, "events", "--help")
	integration.RequireSuccess(t, res)
	for _, flag := range []string{"--type", "--since", "--follow", "--json"} {
		if !strings.Contains(res.Stdout, flag) {
			t.Errorf("missing flag %s in help; got:\n%s", flag, res.Stdout)
		}
	}
}
