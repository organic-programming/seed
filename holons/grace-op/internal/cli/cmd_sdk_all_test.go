package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSdkBuildAllExpandsToOrderedList(t *testing.T) {
	want := []string{"go", "java", "kotlin", "dart", "swift", "python", "csharp", "js", "js-web", "rust", "ruby", "zig", "c", "cpp"}
	if got := sdkAllOrderedLangs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("sdkAllOrderedLangs() = %#v, want %#v", got, want)
	}
}

func TestSdkBuildAllContinuesPastFailure(t *testing.T) {
	var called []string
	results, err := runTestSdkAll(t, sdkAllOptions{
		kind:  sdkAllBuild,
		langs: []string{"go", "java", "kotlin"},
		runSDK: func(_ context.Context, lang string, log io.Writer) error {
			called = append(called, lang)
			if lang == "java" {
				_, _ = io.WriteString(log, "java failed\n")
				return errors.New("boom")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("runSdkAll returned nil error, want failure")
	}
	if want := []string{"go", "java", "kotlin"}; !reflect.DeepEqual(called, want) {
		t.Fatalf("called = %#v, want %#v", called, want)
	}
	if got := statuses(results); !reflect.DeepEqual(got, []sdkAllStatus{sdkAllStatusOK, sdkAllStatusFail, sdkAllStatusOK}) {
		t.Fatalf("statuses = %#v", got)
	}
}

func TestSdkBuildAllSkipsNonCompilable(t *testing.T) {
	var called []string
	results, err := runTestSdkAll(t, sdkAllOptions{
		kind:  sdkAllBuild,
		langs: []string{"go", "java", "kotlin"},
		runSDK: func(_ context.Context, lang string, _ io.Writer) error {
			called = append(called, lang)
			return nil
		},
		skipSDK: func(lang string) ([]string, error) {
			if lang == "java" {
				return []string{"javac not on PATH"}, nil
			}
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("runSdkAll returned error: %v", err)
	}
	if want := []string{"go", "kotlin"}; !reflect.DeepEqual(called, want) {
		t.Fatalf("called = %#v, want %#v", called, want)
	}
	if got := statuses(results); !reflect.DeepEqual(got, []sdkAllStatus{sdkAllStatusOK, sdkAllStatusSkipped, sdkAllStatusOK}) {
		t.Fatalf("statuses = %#v", got)
	}
	if !strings.Contains(results[1].Detail, "javac not on PATH") {
		t.Fatalf("skip detail = %q", results[1].Detail)
	}
}

func TestSdkBuildAllWritesPerSdkLog(t *testing.T) {
	root := t.TempDir()
	_, err := runTestSdkAll(t, sdkAllOptions{
		kind:       sdkAllBuild,
		langs:      []string{"go"},
		logBaseDir: root,
		runSDK: func(_ context.Context, _ string, log io.Writer) error {
			_, _ = io.WriteString(log, "builder output\n")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runSdkAll returned error: %v", err)
	}
	logPath := filepath.Join(root, "20260102T030405Z", "go.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "builder output") {
		t.Fatalf("log %s missing builder output: %q", logPath, string(data))
	}
}

func TestSdkBuildAllWritesSummary(t *testing.T) {
	root := t.TempDir()
	results, err := runTestSdkAll(t, sdkAllOptions{
		kind:       sdkAllBuild,
		langs:      []string{"go", "java", "kotlin"},
		logBaseDir: root,
		runSDK: func(_ context.Context, lang string, _ io.Writer) error {
			if lang == "java" {
				return errors.New("boom")
			}
			return nil
		},
		skipSDK: func(lang string) ([]string, error) {
			if lang == "kotlin" {
				return []string{"blocked"}, nil
			}
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("runSdkAll returned nil error, want failure")
	}
	if len(results) != 3 {
		t.Fatalf("results len = %d, want 3", len(results))
	}
	summaryPath := filepath.Join(root, "20260102T030405Z", "summary.txt")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("summary lines = %d, want 3: %q", len(lines), string(data))
	}
	for _, want := range []string{"OK | go |", "FAIL | java |", "SKIPPED | kotlin |"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("summary missing %q: %q", want, string(data))
		}
	}
}

func TestSdkInstallAllSequentialOrder(t *testing.T) {
	var called []string
	_, err := runTestSdkAll(t, sdkAllOptions{
		kind:  sdkAllInstall,
		langs: sdkAllOrderedLangs(),
		runSDK: func(_ context.Context, lang string, _ io.Writer) error {
			called = append(called, lang)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runSdkAll returned error: %v", err)
	}
	if want := sdkAllOrderedLangs(); !reflect.DeepEqual(called, want) {
		t.Fatalf("called = %#v, want %#v", called, want)
	}
}

func TestSdkBuildRejectsMissingVersion(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "all/missing", args: []string{"all"}},
		{name: "all/blank", args: []string{"all", "--version", "   "}},
		{name: "single/missing", args: []string{"go"}},
		{name: "single/blank", args: []string{"go", "--version", "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newSdkBuildCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tc.args)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("op sdk build %v without --version: want error, got nil", tc.args)
			}
			if !strings.Contains(err.Error(), "--version") {
				t.Fatalf("error %q does not mention --version requirement", err.Error())
			}
		})
	}
}

func TestSdkBuildAllRejectsTargetForCrossOnPureHostSdk(t *testing.T) {
	var called []string
	target := "x86_64-pc-windows-msvc"
	results, err := runTestSdkAll(t, sdkAllOptions{
		kind:   sdkAllBuild,
		target: target,
		langs:  []string{"go", "dart", "js-web"},
		runSDK: func(_ context.Context, lang string, _ io.Writer) error {
			called = append(called, lang)
			return nil
		},
		skipSDK: func(lang string) ([]string, error) {
			if blocker := sdkBuildAllTargetBlocker(lang, target); blocker != "" {
				return []string{blocker}, nil
			}
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("runSdkAll returned error: %v", err)
	}
	if want := []string{"go", "js-web"}; !reflect.DeepEqual(called, want) {
		t.Fatalf("called = %#v, want %#v", called, want)
	}
	if got := statuses(results); !reflect.DeepEqual(got, []sdkAllStatus{sdkAllStatusOK, sdkAllStatusSkipped, sdkAllStatusOK}) {
		t.Fatalf("statuses = %#v", got)
	}
	if !strings.Contains(results[1].Detail, "host-native only") {
		t.Fatalf("skip detail = %q", results[1].Detail)
	}
}

func runTestSdkAll(t *testing.T, opts sdkAllOptions) ([]sdkAllResult, error) {
	t.Helper()
	if opts.logBaseDir == "" {
		opts.logBaseDir = t.TempDir()
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	opts.stdout = &stdout
	opts.stderr = &stderr
	opts.now = fixedSDKAllClock()
	return runSdkAll(context.Background(), opts)
}

func fixedSDKAllClock() func() time.Time {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return func() time.Time {
		current := now
		now = now.Add(time.Second)
		return current
	}
}

func statuses(results []sdkAllResult) []sdkAllStatus {
	out := make([]sdkAllStatus, 0, len(results))
	for _, result := range results {
		out = append(out, result.Status)
	}
	return out
}
