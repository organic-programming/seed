//go:build e2e

package events_test

import (
	"encoding/json"
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

// TestEvents_CLI_LiveObservableJSON starts a real observable holon and
// verifies op events --json drains lifecycle events.
func TestEvents_CLI_LiveObservableJSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	handle := sb.SpawnObservable(t, "gabriel-greeting-go", integration.ObservableOptions{})
	defer handle.Stop(t)

	res := sb.RunOP(t, "events", "gabriel-greeting-go", "--follow=false", "--json")
	integration.RequireSuccess(t, res)
	foundReady := false
	for _, line := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode event json: %v\nline=%s\nraw=%s", err, line, res.Stdout)
		}
		if payload["type"] == "INSTANCE_READY" && payload["slug"] == "gabriel-greeting-go" {
			foundReady = true
		}
	}
	if !foundReady {
		t.Fatalf("events output missing INSTANCE_READY for gabriel-greeting-go:\n%s", res.Stdout)
	}
}
