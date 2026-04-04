package discover_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestDiscover_RPC_ListsWorkspaceHolons(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Discover(ctx, &opv1.DiscoverRequest{RootDir: integration.DefaultWorkspaceDir(t)})
	if err != nil {
		t.Fatalf("rpc Discover: %v", err)
	}
	if len(resp.GetEntries()) == 0 {
		t.Fatal("Discover returned no entries")
	}
}
