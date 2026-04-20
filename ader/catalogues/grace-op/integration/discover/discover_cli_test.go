//go:build e2e

package discover_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestDiscover_CLI_Text(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "discover")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "gabriel-greeting-go")
	integration.RequireContains(t, result.Stdout, "SLUG")
}

func TestDiscover_CLI_JSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	payload := integration.ReadDiscoverJSON(t, sb)
	if len(payload.Entries) == 0 {
		t.Fatal("discover returned no entries")
	}
}

func TestDiscover_CLI_ContainsAvailableHelloWorldHolons(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	sb := integration.NewSandbox(t)
	payload := integration.ReadDiscoverJSON(t, sb)
	found := make(map[string]struct{}, len(payload.Entries))
	for _, entry := range payload.Entries {
		found[entry.Slug] = struct{}{}
	}

	for _, slug := range integration.AvailableHelloWorldSlugs(t, true) {
		if _, ok := found[slug]; !ok {
			t.Fatalf("discover output missing %s", slug)
		}
	}
}
