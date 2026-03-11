package verify

import (
	"strings"
	"testing"
	"time"
)

func TestRunHandlesSuccessAndTimeout(t *testing.T) {
	t.Parallel()

	results := Run([]string{"printf ok", "sleep 1"}, t.TempDir(), 50*time.Millisecond)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Passed || strings.TrimSpace(results[0].Output) != "ok" {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
	if results[1].Passed {
		t.Fatalf("expected timeout failure, got pass")
	}
	if !strings.Contains(results[1].Output, "timed out") {
		t.Fatalf("expected timeout output, got %q", results[1].Output)
	}
}
