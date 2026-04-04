package list_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestList_API_ReturnsWorkspaceIdentities(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.ListIdentities(&opv1.ListIdentitiesRequest{RootDir: integration.DefaultWorkspaceDir(t)})
		if err != nil {
			t.Fatalf("api.ListIdentities: %v", err)
		}
		if len(resp.GetEntries()) == 0 {
			t.Fatal("ListIdentities returned no entries")
		}
	})
}
