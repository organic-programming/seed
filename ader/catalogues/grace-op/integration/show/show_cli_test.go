//go:build e2e

package show_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestShow_CLI_TextByUUIDPrefix(t *testing.T) {
	sb := integration.NewSandbox(t)
	list := integration.ReadListJSON(t, sb)
	if len(list.Entries) == 0 {
		t.Fatal("list returned no entries")
	}
	uuid := list.Entries[0].Identity.UUID
	result := sb.RunOP(t, "show", uuid[:8])
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, list.Entries[0].Identity.GivenName)
	integration.RequireContains(t, result.Stdout, "holon.proto")
}

func TestShow_CLI_JSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	list := integration.ReadListJSON(t, sb)
	if len(list.Entries) == 0 {
		t.Fatal("list returned no entries")
	}
	result := sb.RunOP(t, "--format", "json", "show", list.Entries[0].Identity.UUID)
	integration.RequireSuccess(t, result)
	payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
	identity, ok := payload["identity"].(map[string]any)
	if !ok {
		t.Fatalf("identity = %#v, want object", payload["identity"])
	}
	if identity["uuid"] == "" || identity["givenName"] == "" || identity["familyName"] == "" {
		t.Fatalf("unexpected show payload: %#v", payload)
	}
	filePath, ok := payload["filePath"].(string)
	if !ok || filePath == "" {
		t.Fatalf("filePath = %#v, want non-empty", payload["filePath"])
	}
	if !strings.Contains(filePath, "examples/hello-world/") {
		t.Fatalf("filePath = %#v, want hello-world entry", filePath)
	}
}
