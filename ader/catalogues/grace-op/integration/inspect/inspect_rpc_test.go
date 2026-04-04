package inspect_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInspect_RPC_LocalHolon(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Inspect(ctx, &opv1.InspectRequest{Target: "gabriel-greeting-go"})
	if err != nil {
		t.Fatalf("rpc Inspect: %v", err)
	}
	if len(resp.GetDocument().GetServices()) == 0 || len(resp.GetDocument().GetSequences()) == 0 {
		t.Fatalf("unexpected inspect document: %#v", resp.GetDocument())
	}
}
