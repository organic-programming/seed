//go:build e2e

package clean_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestClean_RPC_RemovesBuildOutputs(t *testing.T) {
	sb := integration.NewSandbox(t)
	build := integration.BuildReportFor(t, sb, "gabriel-greeting-go")
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Clean(ctx, &opv1.LifecycleRequest{Target: "gabriel-greeting-go"})
	if err != nil {
		t.Fatalf("rpc Clean: %v", err)
	}
	if resp.GetReport().GetOperation() != "clean" {
		t.Fatalf("operation = %q, want clean", resp.GetReport().GetOperation())
	}
	integration.RequirePathMissing(t, integration.ReportPath(t, build.Artifact))
}
