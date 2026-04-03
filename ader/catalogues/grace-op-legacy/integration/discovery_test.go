// Discovery tests verify that op discover and op list can find the mirrored
// hello-world workspace and return stable text and JSON output.
package integration

import (
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "discover")
	requireSuccess(t, result)
	requireContains(t, result.Stdout, "gabriel-greeting-go")
	requireContains(t, result.Stdout, "SLUG")
}

func TestDiscoverJSON(t *testing.T) {
	sb := newSandbox(t)
	payload := readDiscoverJSON(t, sb)
	if len(payload.Entries) == 0 {
		t.Fatal("discover returned no entries")
	}
}

func TestDiscoverContainsAvailableHelloWorldHolons(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	payload := readDiscoverJSON(t, sb)
	found := make(map[string]struct{}, len(payload.Entries))
	for _, entry := range payload.Entries {
		found[entry.Slug] = struct{}{}
	}

	for _, slug := range availableHelloWorldSlugs(t, true) {
		if _, ok := found[slug]; !ok {
			t.Fatalf("discover output missing %s", slug)
		}
	}
}

func TestList(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "list")
	requireSuccess(t, result)
	requireContains(t, result.Stdout, "UUID")
	requireContains(t, result.Stdout, "Gabriel")
}

func TestListJSON(t *testing.T) {
	sb := newSandbox(t)
	payload := readListJSON(t, sb)
	if len(payload.Entries) == 0 {
		t.Fatal("list returned no entries")
	}

	wantPaths := make(map[string]struct{})
	for _, slug := range availableHelloWorldSlugs(t, !testing.Short()) {
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
