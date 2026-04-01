// Lifecycle tests cover check, build, test, inspect, release build, and clean
// for every available hello-world holon in the mirrored workspace.
package integration

import (
	"strings"
	"testing"
)

func TestLifecycle_HelloWorldHolons(t *testing.T) {
	for _, spec := range lifecycleHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)

			checkResult := sb.runOP(t, "check", spec.Slug)
			requireSuccess(t, checkResult)

			if spec.Slug != "gabriel-greeting-go" {
				testResult := sb.runOP(t, "test", spec.Slug)
				requireSuccess(t, testResult)
			}

			dryRunText := sb.runOP(t, "build", "--dry-run", spec.Slug)
			requireSuccess(t, dryRunText)
			requireContains(t, dryRunText.Stdout, "Operation: build")
			requireContains(t, dryRunText.Stdout, spec.Slug)

			dryRunJSON := buildDryRunReportFor(t, sb, spec.Slug)
			if dryRunJSON.Operation != "build" {
				t.Fatalf("dry-run operation = %q, want build", dryRunJSON.Operation)
			}
			requireContains(t, strings.Join(dryRunJSON.Notes, "\n"), "dry run")

			buildJSON := buildReportFor(t, sb, spec.Slug)
			if buildJSON.Operation != "build" {
				t.Fatalf("build operation = %q, want build", buildJSON.Operation)
			}
			requirePathExists(t, reportPath(buildJSON.Artifact))

			inspectText := sb.runOP(t, "inspect", spec.Slug)
			requireSuccess(t, inspectText)
			requireContains(t, inspectText.Stdout, spec.Slug)

			inspectJSON := sb.runOP(t, "inspect", spec.Slug, "--json")
			requireSuccess(t, inspectJSON)
			var doc struct {
				Services []struct {
					Name string `json:"name"`
				} `json:"services"`
			}
			doc = decodeJSON[struct {
				Services []struct {
					Name string `json:"name"`
				} `json:"services"`
			}](t, inspectJSON.Stdout)
			if len(doc.Services) == 0 {
				t.Fatalf("inspect %s returned no services", spec.Slug)
			}

			releaseBuild := sb.runOP(t, "--format", "json", "build", "--clean", "--mode", "release", spec.Slug)
			requireSuccess(t, releaseBuild)
			releaseReport := decodeJSON[lifecycleReport](t, releaseBuild.Stdout)
			if releaseReport.BuildMode != "release" {
				t.Fatalf("release build mode = %q, want release", releaseReport.BuildMode)
			}
			requirePathExists(t, reportPath(releaseReport.Artifact))

			cleanResult := sb.runOP(t, "clean", spec.Slug)
			requireSuccess(t, cleanResult)
			requirePathMissing(t, reportPath(releaseReport.Artifact))
		})
	}
}
