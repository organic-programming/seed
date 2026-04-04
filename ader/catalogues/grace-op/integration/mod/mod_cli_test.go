package mod_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestMod_CLI_InitListGraphPullTidyRemove(t *testing.T) {
	sb := integration.NewSandbox(t)
	root := t.TempDir()
	writeModRootFixture(t, root)
	installModRemoteTagsFixture(t)

	depPath := "github.com/example/dep"
	version := "v1.0.0"
	writeCachedDependencyFixture(t, sb, depPath, version)

	opts := integration.RunOptions{SkipDiscoverRoot: true, WorkDir: root}

	initResult := sb.RunOPWithOptions(t, opts, "--format", "json", "mod", "init", "alpha-builder")
	integration.RequireSuccess(t, initResult)
	initPayload := integration.DecodeJSON[map[string]any](t, initResult.Stdout)
	integration.RequireContains(t, initPayload["mod_file"].(string), "holon.mod")

	addResult := sb.RunOPWithOptions(t, opts, "--format", "json", "mod", "add", depPath, version)
	integration.RequireSuccess(t, addResult)
	addPayload := integration.DecodeJSON[map[string]any](t, addResult.Stdout)
	dependency := addPayload["dependency"].(map[string]any)
	if dependency["version"] != version {
		t.Fatalf("version = %#v, want %s", dependency["version"], version)
	}

	listResult := sb.RunOPWithOptions(t, opts, "--format", "json", "mod", "list")
	integration.RequireSuccess(t, listResult)
	listPayload := integration.DecodeJSON[map[string]any](t, listResult.Stdout)
	deps, ok := listPayload["dependencies"].([]any)
	if !ok || len(deps) != 1 {
		t.Fatalf("dependencies = %#v, want 1 entry", listPayload["dependencies"])
	}

	graphResult := sb.RunOPWithOptions(t, opts, "mod", "graph")
	integration.RequireSuccess(t, graphResult)
	integration.RequireContains(t, graphResult.Stdout, "github.com/example/subdep@v0.2.0")

	pullResult := sb.RunOPWithOptions(t, opts, "mod", "pull")
	integration.RequireSuccess(t, pullResult)
	integration.RequireContains(t, pullResult.Stdout, depPath+"@"+version)

	if err := os.WriteFile(filepath.Join(root, "holon.sum"), []byte(depPath+" "+version+" h1:keep\n"+"github.com/example/stale v9.9.9 h1:drop\n"), 0o644); err != nil {
		t.Fatalf("write holon.sum: %v", err)
	}
	tidyResult := sb.RunOPWithOptions(t, opts, "mod", "tidy")
	integration.RequireSuccess(t, tidyResult)
	integration.RequireContains(t, tidyResult.Stdout, "updated")
	sumData, err := os.ReadFile(filepath.Join(root, "holon.sum"))
	if err != nil {
		t.Fatalf("read holon.sum: %v", err)
	}
	if strings.Contains(string(sumData), "github.com/example/stale") {
		t.Fatalf("holon.sum still contains stale dependency: %s", string(sumData))
	}

	updateResult := sb.RunOPWithOptions(t, opts, "mod", "update")
	integration.RequireSuccess(t, updateResult)
	integration.RequireContains(t, updateResult.Stdout, depPath)
	integration.RequireContains(t, updateResult.Stdout, "v1.5.0")

	removeResult := sb.RunOPWithOptions(t, opts, "--format", "json", "mod", "remove", depPath)
	integration.RequireSuccess(t, removeResult)
	removePayload := integration.DecodeJSON[map[string]any](t, removeResult.Stdout)
	if removePayload["path"] != depPath {
		t.Fatalf("path = %#v, want %s", removePayload["path"], depPath)
	}
}
