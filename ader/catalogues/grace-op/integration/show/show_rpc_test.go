//go:build e2e

package show_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestShow_RPC_ReturnsIdentityDetails(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := client.ListIdentities(ctx, &opv1.ListIdentitiesRequest{RootDir: integration.DefaultWorkspaceDir(t)})
	if err != nil {
		t.Fatalf("rpc ListIdentities: %v", err)
	}
	if len(list.GetEntries()) == 0 {
		t.Fatal("ListIdentities returned no entries")
	}
	resp, err := client.ShowIdentity(ctx, &opv1.ShowIdentityRequest{Uuid: list.GetEntries()[0].GetIdentity().GetUuid()})
	if err != nil {
		t.Fatalf("rpc ShowIdentity: %v", err)
	}
	if resp.GetIdentity().GetUuid() == "" || resp.GetFilePath() == "" {
		t.Fatalf("unexpected show response: %#v", resp)
	}
}
