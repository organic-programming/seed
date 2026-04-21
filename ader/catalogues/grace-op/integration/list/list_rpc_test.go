//go:build e2e

package list_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestList_RPC_ReturnsWorkspaceIdentities(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.ListIdentities(ctx, &opv1.ListIdentitiesRequest{RootDir: integration.DefaultWorkspaceDir(t)})
	if err != nil {
		t.Fatalf("rpc ListIdentities: %v", err)
	}
	if len(resp.GetEntries()) == 0 {
		t.Fatal("ListIdentities returned no entries")
	}
}
