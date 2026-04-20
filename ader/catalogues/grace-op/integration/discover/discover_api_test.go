//go:build e2e

package discover_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestDiscover_API_ListsWorkspaceHolons(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.Discover(&opv1.DiscoverRequest{RootDir: integration.DefaultWorkspaceDir(t)})
		if err != nil {
			t.Fatalf("api.Discover: %v", err)
		}
		if len(resp.GetEntries()) == 0 {
			t.Fatal("Discover returned no entries")
		}
		found := false
		for _, entry := range resp.GetEntries() {
			if entry.GetIdentity().GetGivenName() == "Gabriel" && entry.GetRelativePath() == "examples/hello-world/gabriel-greeting-go" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("Discover did not expose gabriel-greeting-go")
		}
	})
}
