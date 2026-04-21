package cli

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/holons"
)

func TestFormatLifecycleReportTextRecursesIntoChildren(t *testing.T) {
	arch := runtime.GOOS + "_" + runtime.GOARCH
	report := holons.Report{
		Operation:   "build",
		Holon:       "parent",
		Dir:         "parent",
		Runner:      "recipe",
		Kind:        "composite",
		BuildTarget: "linux",
		BuildMode:   "release",
		Artifact:    "parent/out/app",
		Commands:    []string{"build_member child"},
		Children: []holons.Report{
			{
				Operation:   "build",
				Holon:       "child",
				Dir:         "parent/child",
				Runner:      "go-module",
				Kind:        "native",
				BuildTarget: "linux",
				BuildMode:   "release",
				Artifact:    "parent/child/.op/build/child.holon",
				Commands:    []string{"go build -o .op/build/child.holon/bin/" + arch + "/child ./cmd/child"},
				Notes:       []string{"dry run — no commands executed"},
			},
		},
	}

	out := formatLifecycleReport(FormatText, report)
	if !strings.Contains(out, "Children:\n  Operation: build\n  Holon: child") {
		t.Fatalf("expected recursive child block, got:\n%s", out)
	}
	if !strings.Contains(out, "  Commands:\n  - go build -o .op/build/child.holon/bin/"+arch+"/child ./cmd/child") {
		t.Fatalf("expected child commands, got:\n%s", out)
	}
}

func TestFormatLifecycleReportJSONIncludesBuildFields(t *testing.T) {
	report := holons.Report{
		Operation:   "build",
		Holon:       "parent",
		BuildTarget: "macos",
		BuildMode:   "debug",
		Artifact:    "examples/app.app",
		Children: []holons.Report{
			{Holon: "child"},
		},
	}

	out := formatLifecycleReport(FormatJSON, report)
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if payload["build_target"] != "macos" {
		t.Fatalf("build_target = %v, want macos", payload["build_target"])
	}
	if payload["build_mode"] != "debug" {
		t.Fatalf("build_mode = %v, want debug", payload["build_mode"])
	}
	if payload["artifact"] != "examples/app.app" {
		t.Fatalf("artifact = %v, want examples/app.app", payload["artifact"])
	}
	children, ok := payload["children"].([]any)
	if !ok || len(children) != 1 {
		t.Fatalf("children = %v, want one child", payload["children"])
	}
}
