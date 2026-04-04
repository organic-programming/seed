package uninstall_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestUninstall_CLI_RemovesInstalledArtifact(t *testing.T) {
	sb := integration.NewSandbox(t)
	report := integration.InstallReportFor(t, sb, "--build", "gabriel-greeting-go")
	integration.RequirePathExists(t, report.Installed)

	uninstallResult := sb.RunOP(t, "--format", "json", "uninstall", "gabriel-greeting-go.holon")
	integration.RequireSuccess(t, uninstallResult)
	uninstallReport := integration.DecodeJSON[integration.InstallReport](t, uninstallResult.Stdout)
	integration.RequirePathMissing(t, uninstallReport.Installed)
}

func TestUninstall_CLI_MissingIsIdempotent(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "uninstall", "nonexistent")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "Operation: uninstall")
}
