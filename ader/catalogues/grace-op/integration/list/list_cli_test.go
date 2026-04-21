//go:build e2e

package list_test

import (
	"path/filepath"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestList_CLI_Text(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "list")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "UUID")
	integration.RequireContains(t, result.Stdout, "Gabriel")
}

func TestList_CLI_JSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	payload := integration.ReadListJSON(t, sb)
	if len(payload.Entries) == 0 {
		t.Fatal("list returned no entries")
	}

	wantPaths := make(map[string]struct{})
	for _, slug := range integration.AvailableHelloWorldSlugs(t, !testing.Short()) {
		wantPaths[filepath.ToSlash(filepath.Join("examples", "hello-world", slug))] = struct{}{}
	}

	foundAny := false
	for _, entry := range payload.Entries {
		if _, ok := wantPaths[entry.RelativePath]; ok {
			foundAny = true
			break
		}
	}
	if !foundAny {
		t.Fatalf("list output did not include any expected hello-world entries: %#v", payload.Entries)
	}
}
