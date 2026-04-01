//go:build windows

// Windows-only artifact tests verify executable naming and install paths in the
// lifecycle reports produced by the real CLI.
package integration

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindows_BuildReportsExeArtifact(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)
			report := buildDryRunReportFor(t, sb, spec.Slug)
			if filepath.Ext(report.Binary) != ".exe" {
				t.Fatalf("binary = %q, want .exe suffix", report.Binary)
			}
		})
	}
}

func TestWindows_InstallProducesExecutablePackage(t *testing.T) {
	sb := newSandbox(t)
	report := installReportFor(t, sb, "--build", "gabriel-greeting-go")
	if !strings.HasSuffix(strings.ToLower(report.Installed), ".holon") {
		t.Fatalf("installed path = %q, want .holon package", report.Installed)
	}
	requirePathExists(t, report.Installed)
}
