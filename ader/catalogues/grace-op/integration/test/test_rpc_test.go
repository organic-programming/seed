//go:build e2e

package test_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestTest_RPC_RepresentativeHolon(t *testing.T) {
	var target string
	for _, spec := range integration.NativeTestHolons(t) {
		if spec.Slug == "matt-calculator-go" && integration.SupportsOPTest(spec) {
			target = spec.Slug
			break
		}
	}
	if target == "" {
		for _, spec := range integration.NativeTestHolons(t) {
			if spec.Slug != "gabriel-greeting-go" && integration.SupportsOPTest(spec) {
				target = spec.Slug
				break
			}
		}
	}
	if target == "" {
		t.Skip("no suitable non-go holon available for op test")
	}

	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Test(ctx, &opv1.LifecycleRequest{Target: target})
	if err != nil {
		t.Fatalf("rpc Test(%s): %v", target, err)
	}
	if resp.GetReport().GetOperation() != "test" {
		t.Fatalf("operation = %q, want test", resp.GetReport().GetOperation())
	}
}
