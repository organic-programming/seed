//go:build e2e

package logs_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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

// TestLogs_CLI_LiveObservableJSON generates real RPC activity and
// verifies op logs --json drains the holon's in-memory log ring.
func TestLogs_CLI_LiveObservableJSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	handle := sb.SpawnObservable(t, "gabriel-greeting-go", integration.ObservableOptions{})
	defer handle.Stop(t)

	for _, name := range []string{"Alice", "Bob"} {
		res := sb.RunOP(t, handle.Address(), "SayHello", `{"name":"`+name+`","lang_code":"en"}`)
		integration.RequireSuccess(t, res)
	}

	var lines []map[string]any
	integration.WaitUntil(t, 10*time.Second, func() bool {
		res := sb.RunOP(t, "logs", "gabriel-greeting-go", "--follow=false", "--json")
		if res.Err != nil {
			return false
		}
		lines = decodeJSONLines(t, res.Stdout)
		for _, line := range lines {
			if line["kind"] == "log" && strings.Contains(asString(line["rpc_method"]), "SayHello") {
				return true
			}
		}
		return false
	})
}

func decodeJSONLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode json log line: %v\nline=%s\nraw=%s", err, line, raw)
		}
		out = append(out, payload)
	}
	return out
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
