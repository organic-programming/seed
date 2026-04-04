package install_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInstall_CLI_BuildAndInstall(t *testing.T) {
	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)

			integration.BuildReportFor(t, sb, spec.Slug)
			report := integration.InstallReportFor(t, sb, spec.Slug)
			integration.RequirePathExists(t, report.Installed)
		})
	}
}

func TestInstall_CLI_WithBuildFlag(t *testing.T) {
	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			report := integration.InstallReportFor(t, sb, "--build", spec.Slug)
			integration.RequirePathExists(t, report.Installed)
		})
	}
}

func TestInstall_CLI_NoBinaryFails(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.CleanHolon(t, sb, "gabriel-greeting-go")

	result := sb.RunOP(t, "install", "gabriel-greeting-go")
	integration.RequireFailure(t, result)
	integration.RequireContains(t, result.Stderr, "artifact not found")
}
