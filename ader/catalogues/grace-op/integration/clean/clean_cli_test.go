package clean_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestClean_CLI_RemovesBuildOutputs(t *testing.T) {
	sb := integration.NewSandbox(t)
	build := integration.BuildReportFor(t, sb, "gabriel-greeting-go")
	integration.RequirePathExists(t, integration.ReportPath(t, build.Artifact))

	result := sb.RunOP(t, "--format", "json", "clean", "gabriel-greeting-go")
	integration.RequireSuccess(t, result)
	report := integration.DecodeJSON[integration.LifecycleReport](t, result.Stdout)
	if report.Operation != "clean" {
		t.Fatalf("operation = %q, want clean", report.Operation)
	}
	integration.RequirePathMissing(t, integration.ReportPath(t, build.Artifact))
}

func TestClean_CLI_IsIdempotent(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "clean", "gabriel-greeting-go")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "Operation: clean")
}
