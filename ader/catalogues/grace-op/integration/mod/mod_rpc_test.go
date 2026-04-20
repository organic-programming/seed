//go:build e2e

package mod_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestMod_RPC_InitAddListRemove(t *testing.T) {
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

		client, cleanup := integration.SetupSandboxStdioOPClientAt(t, sb, root)
		defer cleanup()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		initResp, err := client.ModInit(ctx, &opv1.ModInitRequest{HolonPath: "alpha-builder"})
		if err != nil {
			t.Fatalf("rpc ModInit: %v", err)
		}
		if filepath.Base(initResp.GetModFile()) != "holon.mod" {
			t.Fatalf("mod file basename = %q, want holon.mod", filepath.Base(initResp.GetModFile()))
		}

		addResp, err := client.ModAdd(ctx, &opv1.ModAddRequest{Module: depPath, Version: version})
		if err != nil {
			t.Fatalf("rpc ModAdd: %v", err)
		}
		if addResp.GetDependency().GetVersion() != version {
			t.Fatalf("version = %q, want %q", addResp.GetDependency().GetVersion(), version)
		}

		listResp, err := client.ModList(ctx, &opv1.ModListRequest{})
		if err != nil {
			t.Fatalf("rpc ModList: %v", err)
		}
		if len(listResp.GetDependencies()) != 1 {
			t.Fatalf("dependencies = %d, want 1", len(listResp.GetDependencies()))
		}

		removeResp, err := client.ModRemove(ctx, &opv1.ModRemoveRequest{Module: depPath})
		if err != nil {
			t.Fatalf("rpc ModRemove: %v", err)
		}
		if removeResp.GetPath() != depPath {
			t.Fatalf("path = %q, want %q", removeResp.GetPath(), depPath)
		}
	})
}
