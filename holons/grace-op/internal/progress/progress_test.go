package progress

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriterNonTTYPrintsOnlyPhaseChanges(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	now := base
	w := newWriter(&buf, false, nil, func() time.Time { return now }, time.Second)

	w.Print("checking manifest...")
	w.Print("checking manifest...")
	now = now.Add(2 * time.Second)
	w.Print("validating prerequisites...")
	now = now.Add(1 * time.Second)
	w.Freeze("building demo… ✓")

	got := buf.String()
	if strings.Count(got, "checking manifest...\n") != 1 {
		t.Fatalf("expected one checking line, got %q", got)
	}
	if !strings.Contains(got, "00:00:02 validating prerequisites...\n") {
		t.Fatalf("output missing updated phase: %q", got)
	}
	if !strings.Contains(got, "00:00:03 building demo… ✓\n") {
		t.Fatalf("output missing frozen success line: %q", got)
	}
}

func TestWriterTTYRepaintsCurrentLineOnTick(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	now := base
	w := newWriter(&buf, true, nil, func() time.Time { return now }, time.Second)

	w.Print("building...")
	now = now.Add(5 * time.Second)
	w.tick()
	w.Freeze("building… ✓")

	got := buf.String()
	if !strings.Contains(got, "\r"+clearToEOL+"00:00:00 building...") {
		t.Fatalf("tty output missing initial repaint: %q", got)
	}
	if !strings.Contains(got, "\r"+clearToEOL+"00:00:05 building...") {
		t.Fatalf("tty output missing tick repaint: %q", got)
	}
	if !strings.Contains(got, "\r"+clearToEOL+"00:00:05 building… ✓\n") {
		t.Fatalf("tty output missing frozen line: %q", got)
	}
}

func TestWriterTTYClearRemovesFrozenAndLiveLines(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	now := base
	w := newWriter(&buf, true, nil, func() time.Time { return now }, time.Second)

	w.Print("building alpha… linking")
	w.Freeze("building alpha… ✓")
	now = now.Add(2 * time.Second)
	w.Print("building beta… packaging")
	w.Clear()

	got := buf.String()
	if !strings.Contains(got, "\r"+clearToEOL+"00:00:00 building alpha… ✓\n") {
		t.Fatalf("tty output missing frozen dependency line: %q", got)
	}
	if !strings.Contains(got, moveUpLine+"\r"+clearToEOL) {
		t.Fatalf("tty clear missing move-up erase sequence: %q", got)
	}
}

func TestChildReporterSharesTimer(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	now := base
	w := newWriter(&buf, false, nil, func() time.Time { return now }, time.Second)

	child := w.Child()
	now = now.Add(1 * time.Second)
	child.Step("go build ./cmd/demo")

	got := buf.String()
	if !strings.Contains(got, "00:00:01    go build ./cmd/demo\n") {
		t.Fatalf("child output missing shared timer/indentation: %q", got)
	}
}

func TestBuildReporterWithStartUsesCustomTimer(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	buildStart := base.Add(30 * time.Second)
	now := buildStart.Add(4 * time.Second)
	w := newWriter(&buf, false, nil, func() time.Time { return now }, time.Second)

	reporter := NewBuildReporterWithStart(w, "demo", buildStart)
	reporter.Step("compiling")

	got := buf.String()
	if !strings.Contains(got, "00:00:04 building demo… compiling\n") {
		t.Fatalf("build reporter output missing custom timer origin: %q", got)
	}
}

func TestSilenceProducesNoOutput(t *testing.T) {
	var buf bytes.Buffer
	w := Silence()
	w.w = &buf
	w.Print("ignored")
	w.Freeze("ignored")
	if buf.Len() != 0 {
		t.Fatalf("silent writer wrote output: %q", buf.String())
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := FormatTimer(3661 * time.Second); got != "01:01:01" {
		t.Fatalf("FormatTimer = %q, want %q", got, "01:01:01")
	}
	if got := FormatElapsed(3500 * time.Millisecond); got != "4s" {
		t.Fatalf("FormatElapsed = %q, want %q", got, "4s")
	}
}

func TestWriterKeepAsPrintsFinalSuccessLine(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	now := base
	w := newWriter(&buf, false, nil, func() time.Time { return now }, time.Second)

	w.Print("building demo… linking")
	now = now.Add(9 * time.Second)
	w.KeepAs("built demo… ✓")

	got := buf.String()
	if !strings.Contains(got, "00:00:00 building demo… linking\n") {
		t.Fatalf("output missing initial phase line: %q", got)
	}
	if !strings.Contains(got, "00:00:09 built demo… ✓\n") {
		t.Fatalf("output missing final built line with elapsed time: %q", got)
	}
}

func TestWriterTTYTruncatesLongLinesToAvoidWrap(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	now := base
	w := newWriter(&buf, true, func() int { return 24 }, func() time.Time { return now }, time.Second)

	w.Print("building gabriel-greeting-swift… swift build --build-path /very/long/path")
	now = now.Add(2 * time.Second)
	w.tick()

	got := buf.String()
	initial := fitTTYLine("00:00:00 building gabriel-greeting-swift… swift build --build-path /very/long/path", 24)
	ticked := fitTTYLine("00:00:02 building gabriel-greeting-swift… swift build --build-path /very/long/path", 24)
	if strings.Contains(got, "/very/long/path") {
		t.Fatalf("tty output should truncate long lines to avoid wrapping: %q", got)
	}
	if !strings.Contains(got, initial) {
		t.Fatalf("tty output missing truncated initial line: %q", got)
	}
	if !strings.Contains(got, ticked) {
		t.Fatalf("tty output missing truncated tick line: %q", got)
	}
}
