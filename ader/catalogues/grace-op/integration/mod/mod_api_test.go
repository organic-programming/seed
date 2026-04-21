//go:build e2e

package mod_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestMod_API_InitAddListRemove(t *testing.T) {
	sb := integration.NewSandbox(t)
	root := t.TempDir()
	writeModRootFixture(t, root)
	installModRemoteTagsFixture(t)

	depPath := "github.com/example/dep"
	version := "v1.0.0"
	writeCachedDependencyFixture(t, sb, depPath, version)

	integration.WithSandboxEnv(t, sb, func() {
		original, _ := os.Getwd()
		defer func() { _ = os.Chdir(original) }()
		if err := os.Chdir(root); err != nil {
			t.Fatalf("Chdir(%s): %v", root, err)
		}

		initResp, err := api.ModInit(&opv1.ModInitRequest{HolonPath: "alpha-builder"})
		if err != nil {
			t.Fatalf("api.ModInit: %v", err)
		}
		if filepath.Base(initResp.GetModFile()) != "holon.mod" {
			t.Fatalf("mod file basename = %q, want holon.mod", filepath.Base(initResp.GetModFile()))
		}

		addResp, err := api.ModAdd(&opv1.ModAddRequest{Module: depPath, Version: version})
		if err != nil {
			t.Fatalf("api.ModAdd: %v", err)
		}
		if addResp.GetDependency().GetVersion() != version {
			t.Fatalf("version = %q, want %q", addResp.GetDependency().GetVersion(), version)
		}

		listResp, err := api.ModList(&opv1.ModListRequest{})
		if err != nil {
			t.Fatalf("api.ModList: %v", err)
		}
		if len(listResp.GetDependencies()) != 1 {
			t.Fatalf("dependencies = %d, want 1", len(listResp.GetDependencies()))
		}

		removeResp, err := api.ModRemove(&opv1.ModRemoveRequest{Module: depPath})
		if err != nil {
			t.Fatalf("api.ModRemove: %v", err)
		}
		if removeResp.GetPath() != depPath {
			t.Fatalf("path = %q, want %q", removeResp.GetPath(), depPath)
		}
	})
}
