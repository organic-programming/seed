package check_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestCheck_RPC_GabrielGreetingGo(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Check(ctx, &opv1.LifecycleRequest{Target: "gabriel-greeting-go"})
	if err != nil {
		t.Fatalf("rpc Check: %v", err)
	}
	if resp.GetReport().GetOperation() != "check" {
		t.Fatalf("operation = %q, want check", resp.GetReport().GetOperation())
	}
}
