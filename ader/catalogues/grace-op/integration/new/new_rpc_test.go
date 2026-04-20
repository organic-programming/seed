//go:build e2e

package new_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestNew_RPC_CreateIdentity(t *testing.T) {
	sb := integration.NewSandbox(t)
	root := t.TempDir()
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

		resp, err := client.CreateIdentity(ctx, &opv1.CreateIdentityRequest{
			GivenName:  "Alpha",
			FamilyName: "Builder",
			Motto:      "Builds holons.",
			Composer:   "test",
			Clade:      opv1.Clade_DETERMINISTIC_IO_BOUND,
			Lang:       "go",
		})
		if err != nil {
			t.Fatalf("rpc CreateIdentity: %v", err)
		}
		if resp.GetIdentity().GetUuid() == "" {
			t.Fatalf("unexpected create identity response: %#v", resp)
		}
		createdPath := filepath.Join(root, "holons", "alpha-builder", "holon.proto")
		if _, err := os.Stat(createdPath); err != nil {
			t.Fatalf("created holon manifest missing: %v", err)
		}
	})
}
