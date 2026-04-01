// Mod command tests initialize and inspect a minimal module workspace without
// depending on discovery from the mirrored hello-world tree.
package integration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMod_InitListTidy(t *testing.T) {
	sb := newSandbox(t)

	root := filepath.Join(sb.Root, "alpha")
	if err := os.MkdirAll(filepath.Join(root, "api", "v1"), 0o755); err != nil {
		t.Fatalf("mkdir mod root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", "v1", "holon.proto"), []byte("syntax = \"proto3\";\npackage sample.v1;\n"), 0o644); err != nil {
		t.Fatalf("write holon.proto: %v", err)
	}

	initResult := sb.runOPWithOptions(t, runOptions{SkipDiscoverRoot: true, WorkDir: root}, "--format", "json", "mod", "init", "alpha")
	requireSuccess(t, initResult)
	initPayload := decodeJSON[map[string]any](t, initResult.Stdout)
	requireContains(t, initPayload["mod_file"].(string), "holon.mod")

	listResult := sb.runOPWithOptions(t, runOptions{SkipDiscoverRoot: true, WorkDir: root}, "--format", "json", "mod", "list")
	requireSuccess(t, listResult)
	listPayload := decodeJSON[map[string]any](t, listResult.Stdout)
	deps, ok := listPayload["dependencies"].([]any)
	if !ok || len(deps) != 0 {
		t.Fatalf("dependencies = %#v, want empty", listPayload["dependencies"])
	}

	tidyResult := sb.runOPWithOptions(t, runOptions{SkipDiscoverRoot: true, WorkDir: root}, "--format", "json", "mod", "tidy")
	requireSuccess(t, tidyResult)
	tidyPayload := decodeJSON[map[string]any](t, tidyResult.Stdout)
	requireContains(t, tidyPayload["sum_file"].(string), "holon.sum")
}
