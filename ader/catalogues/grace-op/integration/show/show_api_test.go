//go:build e2e

package show_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestShow_API_ReturnsIdentityDetails(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		list, err := api.ListIdentities(&opv1.ListIdentitiesRequest{RootDir: integration.DefaultWorkspaceDir(t)})
		if err != nil {
			t.Fatalf("api.ListIdentities: %v", err)
		}
		if len(list.GetEntries()) == 0 {
			t.Fatal("ListIdentities returned no entries")
		}
		resp, err := api.ShowIdentity(&opv1.ShowIdentityRequest{Uuid: list.GetEntries()[0].GetIdentity().GetUuid()})
		if err != nil {
			t.Fatalf("api.ShowIdentity: %v", err)
		}
		if resp.GetIdentity().GetUuid() == "" || resp.GetFilePath() == "" {
			t.Fatalf("unexpected show response: %#v", resp)
		}
	})
}
