// Install tests build, install, and uninstall hello-world holons and check the
// expected failure paths around missing artifacts.
package integration

import (
	"testing"
)

func TestInstall_BuildInstallUninstall(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)

			buildReportFor(t, sb, spec.Slug)
			report := installReportFor(t, sb, spec.Slug)
			requirePathExists(t, report.Installed)

			uninstallResult := sb.runOP(t, "--format", "json", "uninstall", spec.Slug+".holon")
			requireSuccess(t, uninstallResult)
			uninstallReport := decodeJSON[installReport](t, uninstallResult.Stdout)
			requirePathMissing(t, uninstallReport.Installed)
		})
	}
}

func TestInstall_WithBuildFlag(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)
			report := installReportFor(t, sb, "--build", spec.Slug)
			requirePathExists(t, report.Installed)
		})
	}
}

func TestInstall_NoBinaryFails(t *testing.T) {
	sb := newSandbox(t)
	cleanHolon(t, sb, "gabriel-greeting-go")

	result := sb.runOP(t, "install", "gabriel-greeting-go")
	requireFailure(t, result)
	requireContains(t, result.Stderr, "artifact not found")
}

func TestUninstall_MissingIsIdempotent(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "uninstall", "nonexistent")
	requireSuccess(t, result)
	requireContains(t, result.Stdout, "Operation: uninstall")
}
