package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
)

func TestObservedRenderersIncludeChain(t *testing.T) {
	chain := []string{"gabriel-greeting-app-flutter", "gabriel-greeting-go"}
	record := testCLIRecord("hello", "", chain)

	logText := captureObservabilityStdout(t, func() {
		renderLogRecord(record, false)
	})
	if !strings.Contains(logText, "chain=gabriel-greeting-app-flutter>gabriel-greeting-go") {
		t.Fatalf("log text missing chain annotation: %s", logText)
	}

	logJSON := logRecordJSON(record)
	if got := logJSON["chain"].([]string); len(got) != 2 || got[1] != "gabriel-greeting-go" {
		t.Fatalf("log JSON chain = %#v", logJSON["chain"])
	}

	event := testCLIRecord("instance.ready", "instance.ready", chain)
	eventText := captureObservabilityStdout(t, func() {
		renderEvent(event, false)
	})
	if !strings.Contains(eventText, "chain=gabriel-greeting-app-flutter>gabriel-greeting-go") {
		t.Fatalf("event text missing chain annotation: %s", eventText)
	}

	eventJSONText := captureObservabilityStdout(t, func() {
		renderEvent(event, true)
	})
	var eventJSON map[string]any
	if err := json.Unmarshal([]byte(eventJSONText), &eventJSON); err != nil {
		t.Fatalf("decode event JSON: %v\n%s", err, eventJSONText)
	}
	if got, ok := eventJSON["chain"].([]any); !ok || len(got) != 2 {
		t.Fatalf("event JSON chain = %#v", eventJSON["chain"])
	}
}

func testCLIRecord(body, eventName string, chain []string) *v1.LogRecord {
	now := time.Now()
	return &v1.LogRecord{
		TimeUnixNano:         uint64(now.UnixNano()),
		ObservedTimeUnixNano: uint64(now.UnixNano()),
		SeverityNumber:       v1.SeverityNumber_SEVERITY_NUMBER_INFO,
		SeverityText:         "INFO",
		Body:                 &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: body}},
		Attributes: []*v1.KeyValue{
			{Key: "holons.slug", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "gabriel-greeting-go"}}},
			{Key: "holons.instance_uid", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "member-1"}}},
		},
		EventName: eventName,
		Chain:     chain,
	}
}

func TestCandidateRunRootsFollowsInstancesPathResolution(t *testing.T) {
	for _, tc := range []struct {
		name  string
		all   bool
		setup func(t *testing.T) []string
	}{
		{
			name: "default uses existing OP_RUN_DIR",
			setup: func(t *testing.T) []string {
				runDir := t.TempDir()
				t.Setenv("OP_RUN_DIR", runDir)
				t.Setenv("OPPATH", "")
				t.Setenv("OPROOT", "")
				chdir(t, t.TempDir())
				return []string{runDir}
			},
		},
		{
			name: "default falls back to OPPATH run",
			setup: func(t *testing.T) []string {
				opPath := t.TempDir()
				runDir := filepath.Join(opPath, "run")
				mkdir(t, runDir)
				t.Setenv("OP_RUN_DIR", "")
				t.Setenv("OPPATH", opPath)
				t.Setenv("OPROOT", "")
				chdir(t, t.TempDir())
				return []string{runDir}
			},
		},
		{
			name: "default falls back to OPROOT project run",
			setup: func(t *testing.T) []string {
				opRoot := t.TempDir()
				runDir := filepath.Join(opRoot, ".op", "run")
				mkdir(t, runDir)
				t.Setenv("OP_RUN_DIR", "")
				t.Setenv("OPPATH", "")
				t.Setenv("OPROOT", opRoot)
				chdir(t, t.TempDir())
				return []string{runDir}
			},
		},
		{
			name: "default falls back to cwd project run",
			setup: func(t *testing.T) []string {
				cwd := t.TempDir()
				runDir := filepath.Join(cwd, ".op", "run")
				mkdir(t, runDir)
				t.Setenv("OP_RUN_DIR", "")
				t.Setenv("OPPATH", "")
				t.Setenv("OPROOT", "")
				actualCWD := chdir(t, cwd)
				return []string{filepath.Join(actualCWD, ".op", "run")}
			},
		},
		{
			name: "default returns empty when no candidates exist",
			setup: func(t *testing.T) []string {
				t.Setenv("OP_RUN_DIR", "")
				t.Setenv("OPPATH", "")
				t.Setenv("OPROOT", "")
				chdir(t, t.TempDir())
				return nil
			},
		},
		{
			name: "all returns every configured candidate",
			all:  true,
			setup: func(t *testing.T) []string {
				cwd := t.TempDir()
				opRunDir := filepath.Join(t.TempDir(), "missing-run")
				opPath := filepath.Join(t.TempDir(), "missing-oppath")
				opRoot := filepath.Join(t.TempDir(), "missing-oproot")
				t.Setenv("OP_RUN_DIR", opRunDir)
				t.Setenv("OPPATH", opPath)
				t.Setenv("OPROOT", opRoot)
				actualCWD := chdir(t, cwd)
				return []string{
					opRunDir,
					filepath.Join(opPath, "run"),
					filepath.Join(opRoot, ".op", "run"),
					filepath.Join(actualCWD, ".op", "run"),
				}
			},
		},
		{
			name: "default skips nonexistent OP_RUN_DIR",
			setup: func(t *testing.T) []string {
				opPath := t.TempDir()
				runDir := filepath.Join(opPath, "run")
				mkdir(t, runDir)
				t.Setenv("OP_RUN_DIR", filepath.Join(t.TempDir(), "missing-run"))
				t.Setenv("OPPATH", opPath)
				t.Setenv("OPROOT", "")
				chdir(t, t.TempDir())
				return []string{runDir}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.setup(t)
			got := candidateRunRoots(tc.all)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("candidateRunRoots(%v) = %#v, want %#v", tc.all, got, want)
			}
		})
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func chdir(t *testing.T, path string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(path); err != nil {
		t.Fatalf("chdir %s: %v", path, err)
	}
	actual, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd after chdir: %v", err)
	}
	return actual
}

func captureObservabilityStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = old
	})

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return buf.String()
}
