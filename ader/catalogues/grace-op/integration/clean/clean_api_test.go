//go:build e2e

package clean_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestClean_API_RemovesBuildOutputs(t *testing.T) {
	sb := integration.NewSandbox(t)
	build := integration.BuildReportFor(t, sb, "gabriel-greeting-go")
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.Clean(&opv1.LifecycleRequest{Target: "gabriel-greeting-go"})
		if err != nil {
			t.Fatalf("api.Clean: %v", err)
		}
		if resp.GetReport().GetOperation() != "clean" {
			t.Fatalf("operation = %q, want clean", resp.GetReport().GetOperation())
		}
	})
	integration.RequirePathMissing(t, integration.ReportPath(t, build.Artifact))
}
