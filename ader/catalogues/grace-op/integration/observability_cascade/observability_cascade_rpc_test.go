//go:build e2e

package observability_cascade_test

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

type cascadeReport struct {
	Ticks int `json:"ticks"`
	Pass  int `json:"pass"`
	Fail  int `json:"fail"`
}

type multiPatternReport struct {
	TotalPass int `json:"totalPass"`
	TotalFail int `json:"totalFail"`
}

func TestObservabilityCascade_RPCMatrix(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)
	if runtime.GOOS != "darwin" {
		t.Skip("observability-cascade composite validation currently targets macOS hosts")
	}

	sb := integration.NewSandbox(t)
	for _, lang := range selectedCascadeLanguages(t) {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			slug := "observability-cascade-" + lang
			build := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 30 * time.Minute}, "build", slug, "--install")
			integration.RequireSuccess(t, build)

			assertCascadeReport(t, sb, slug, "RunDefault", 12)
			assertCascadeReport(t, sb, slug, "RunLiveStream", 12)
			assertMultiPatternReport(t, sb, slug, 36)
		})
	}
}

func selectedCascadeLanguages(t *testing.T) []string {
	t.Helper()
	languages := []string{
		"go",
		"rust",
		"dart",
		"python",
		"ruby",
		"node",
		"java",
		"kotlin",
		"csharp",
		"swift",
		"c",
		"cpp",
		"zig",
	}
	filter := strings.TrimSpace(os.Getenv("OBSERVABILITY_CASCADE_LANG"))
	if filter == "" || filter == "all" {
		return languages
	}
	for _, lang := range languages {
		if filter == lang || filter == "observability-cascade-"+lang {
			return []string{lang}
		}
	}
	t.Fatalf("unknown OBSERVABILITY_CASCADE_LANG=%q", filter)
	return nil
}

func assertCascadeReport(t *testing.T, sb *integration.Sandbox, slug, method string, expectedTicks int) {
	t.Helper()

	result := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 15 * time.Minute}, "invoke", slug, method, "{}", "-f", "json")
	integration.RequireSuccess(t, result)
	report := integration.DecodeJSON[cascadeReport](t, result.Stdout)
	if report.Ticks != expectedTicks || report.Pass != report.Ticks || report.Fail != 0 {
		t.Fatalf("%s %s = %+v, want pass == ticks == %d and fail == 0", slug, method, report, expectedTicks)
	}
}

func assertMultiPatternReport(t *testing.T, sb *integration.Sandbox, slug string, expectedPass int) {
	t.Helper()

	result := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 30 * time.Minute}, "invoke", slug, "RunMultiPattern", "{}", "-f", "json")
	integration.RequireSuccess(t, result)
	report := integration.DecodeJSON[multiPatternReport](t, result.Stdout)
	if report.TotalPass != expectedPass || report.TotalFail != 0 {
		t.Fatalf("%s RunMultiPattern = %+v, want totalPass == %d and totalFail == 0", slug, report, expectedPass)
	}
}
