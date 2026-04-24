package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestObservedRenderersIncludeChain(t *testing.T) {
	chain := []*v1.ChainHop{
		{Slug: "gabriel-greeting-app-flutter", InstanceUid: "root-1"},
		{Slug: "gabriel-greeting-go", InstanceUid: "member-1"},
	}

	logText := captureObservabilityStdout(t, func() {
		renderLogEntry(&v1.LogEntry{
			Ts:          timestamppb.Now(),
			Level:       v1.LogLevel_INFO,
			Slug:        "gabriel-greeting-go",
			InstanceUid: "member-1",
			Message:     "hello",
			Chain:       chain,
		}, false)
	})
	if !strings.Contains(logText, "chain=gabriel-greeting-app-flutter/root-1>gabriel-greeting-go/member-1") {
		t.Fatalf("log text missing chain annotation: %s", logText)
	}

	logJSON := logEntryJSON(&v1.LogEntry{Slug: "gabriel-greeting-go", InstanceUid: "member-1", Message: "hello", Chain: chain})
	if got := logJSON["chain"].([]map[string]string); len(got) != 2 || got[1]["slug"] != "gabriel-greeting-go" {
		t.Fatalf("log JSON chain = %#v", logJSON["chain"])
	}

	eventText := captureObservabilityStdout(t, func() {
		renderEvent(&v1.EventInfo{
			Ts:          timestamppb.Now(),
			Type:        v1.EventType_INSTANCE_READY,
			Slug:        "gabriel-greeting-go",
			InstanceUid: "member-1",
			Chain:       chain,
		}, false)
	})
	if !strings.Contains(eventText, "chain=gabriel-greeting-app-flutter/root-1>gabriel-greeting-go/member-1") {
		t.Fatalf("event text missing chain annotation: %s", eventText)
	}

	eventJSONText := captureObservabilityStdout(t, func() {
		renderEvent(&v1.EventInfo{
			Ts:          timestamppb.Now(),
			Type:        v1.EventType_INSTANCE_READY,
			Slug:        "gabriel-greeting-go",
			InstanceUid: "member-1",
			Chain:       chain,
		}, true)
	})
	var eventJSON map[string]any
	if err := json.Unmarshal([]byte(eventJSONText), &eventJSON); err != nil {
		t.Fatalf("decode event JSON: %v\n%s", err, eventJSONText)
	}
	if got, ok := eventJSON["chain"].([]any); !ok || len(got) != 2 {
		t.Fatalf("event JSON chain = %#v", eventJSON["chain"])
	}
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
