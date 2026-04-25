//go:build e2e

package metrics_test

import (
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestMetrics_CLI_NoInstance(t *testing.T) {
	sb := integration.NewSandbox(t)
	res := sb.RunOP(t, "metrics", "no-such-slug")
	integration.RequireFailure(t, res)
	if !strings.Contains(res.Stderr, "no running instance") {
		t.Fatalf("expected not-found message; stderr:\n%s", res.Stderr)
	}
}

func TestMetrics_CLI_Help(t *testing.T) {
	sb := integration.NewSandbox(t)
	res := sb.RunOP(t, "metrics", "--help")
	integration.RequireSuccess(t, res)
	for _, flag := range []string{"--prom", "--json", "--prefix", "--include-session-rollup"} {
		if !strings.Contains(res.Stdout, flag) {
			t.Errorf("missing flag %s in help; got:\n%s", flag, res.Stdout)
		}
	}
}

// TestMetrics_CLI_LiveObservableJSON generates real RPC activity and
// verifies op metrics --json exposes baseline RPC counters.
func TestMetrics_CLI_LiveObservableJSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	handle := sb.SpawnObservable(t, "gabriel-greeting-go", integration.ObservableOptions{})
	defer handle.Stop(t)

	for _, name := range []string{"Alice", "Bob"} {
		res := sb.RunOP(t, handle.Address(), "SayHello", `{"name":"`+name+`","lang_code":"en"}`)
		integration.RequireSuccess(t, res)
	}

	integration.WaitUntil(t, 10*time.Second, func() bool {
		res := sb.RunOP(t, "metrics", "gabriel-greeting-go", "--json")
		return res.Err == nil &&
			strings.Contains(res.Stdout, "holon_session_rpc_total") &&
			strings.Contains(res.Stdout, "SayHello")
	})
}
