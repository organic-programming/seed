package version_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestVersion_RPC_ReturnsBanner(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Version(ctx, &opv1.VersionRequest{})
	if err != nil {
		t.Fatalf("rpc Version: %v", err)
	}
	if resp.GetName() != "op" || resp.GetBanner() == "" {
		t.Fatalf("unexpected version response: %#v", resp)
	}
}
