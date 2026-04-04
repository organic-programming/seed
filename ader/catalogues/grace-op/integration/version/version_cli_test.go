package version_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestVersion_CLI_Text(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "version")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "op")
}

func TestVersion_CLI_JSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "--format", "json", "version")
	integration.RequireSuccess(t, result)
	payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
	if payload["name"] != "op" {
		t.Fatalf("name = %#v, want op", payload["name"])
	}
	if payload["banner"] == "" {
		t.Fatalf("banner = %#v, want non-empty", payload["banner"])
	}
}
