//go:build e2e

package ps_test

import (
	"encoding/json"
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

// TestPs_CLI_LiveObservableJSON starts one observability-enabled Go
// holon and verifies op ps --json reports that live instance from the
// sandbox run registry.
func TestPs_CLI_LiveObservableJSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	handle := sb.SpawnObservable(t, "gabriel-greeting-go", integration.ObservableOptions{})
	defer handle.Stop(t)

	res := sb.RunOP(t, "ps", "--json")
	integration.RequireSuccess(t, res)
	var rows []map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &rows); err != nil {
		t.Fatalf("decode ps json: %v\n%s", err, res.Stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("op ps rows len=%d, want 1\n%s", len(rows), res.Stdout)
	}
	row := rows[0]
	if row["slug"] != "gabriel-greeting-go" || row["uid"] != handle.UID() {
		t.Fatalf("row identity = %#v, want slug gabriel-greeting-go uid %s", row, handle.UID())
	}
	if row["address"] == "" || row["alive"] != true {
		t.Fatalf("row should be alive with address: %#v", row)
	}
}
