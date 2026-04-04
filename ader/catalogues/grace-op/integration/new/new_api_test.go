package new_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestNew_API_CreateIdentity(t *testing.T) {
	sb := integration.NewSandbox(t)
	root := t.TempDir()
	integration.WithSandboxEnv(t, sb, func() {
		original, _ := os.Getwd()
		defer func() { _ = os.Chdir(original) }()
		if err := os.Chdir(root); err != nil {
			t.Fatalf("Chdir(%s): %v", root, err)
		}
		resp, err := api.CreateIdentity(&opv1.CreateIdentityRequest{
			GivenName:  "Alpha",
			FamilyName: "Builder",
			Motto:      "Builds holons.",
			Composer:   "test",
			Clade:      opv1.Clade_DETERMINISTIC_IO_BOUND,
			Lang:       "go",
		})
		if err != nil {
			t.Fatalf("api.CreateIdentity: %v", err)
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
