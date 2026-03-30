package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

const (
	clearToEOL = "\x1b[K"
	moveUpLine = "\x1b[1A"
)

type Reporter interface {
	Step(msg string)
	Stepf(format string, args ...any)
	Child() Reporter
}

type Writer struct {
	mu sync.Mutex

	w      io.Writer
	tty    bool
	silent bool
	width  func() int

	start time.Time
	now   func() time.Time

	tickEvery     time.Duration
	tickerRunning bool
	tickerStop    chan struct{}

	currentMsg     string
	currentStart   time.Time
	currentVisible bool
	frozenLines    int
	lastPlain      string
}

type scopedReporter struct {
	writer *Writer
	prefix string
	indent string
	label  string
	start  time.Time
}

func New(w io.Writer) *Writer {
	if w == nil {
		return Silence()
	}

	isTTY := false
	var width func() int
	if file, ok := w.(*os.File); ok && file != nil {
		isTTY = term.IsTerminal(int(file.Fd()))
		width = func() int {
			cols, _, err := term.GetSize(int(file.Fd()))
			if err != nil || cols <= 0 {
				return 0
			}
			return cols
		}
	}

	return newWriter(w, isTTY, width, time.Now, time.Second)
}

func Silence() *Writer {
	return &Writer{
		silent:    true,
		start:     time.Now(),
		now:       time.Now,
		tickEvery: time.Second,
	}
}

func NewBuildReporter(w *Writer, label string) Reporter {
	return NewBuildReporterWithStart(w, label, time.Time{})
}

func NewBuildReporterWithStart(w *Writer, label string, start time.Time) Reporter {
	if w == nil {
		return Silence()
	}
	return &scopedReporter{
		writer: w,
		prefix: buildPrefix(label),
		label:  strings.TrimSpace(label),
		start:  start,
	}
}

func WriterFromReporter(reporter Reporter) *Writer {
	switch v := reporter.(type) {
	case nil:
		return nil
	case *Writer:
		return v
	case *scopedReporter:
		return v.writer
	default:
		return nil
	}
}

func BuildReporterLabel(reporter Reporter) string {
	if scoped, ok := reporter.(*scopedReporter); ok {
		return scoped.label
	}
	return ""
}

func (pw *Writer) Step(msg string) {
	pw.Print(msg)
}

func (pw *Writer) Stepf(format string, args ...any) {
	pw.Print(fmt.Sprintf(format, args...))
}

func (pw *Writer) Child() Reporter {
	if pw == nil {
		return Silence()
	}
	return &scopedReporter{writer: pw, indent: "  "}
}

func (pw *Writer) Done(msg string, err error) {
	if pw == nil || pw.silent {
		return
	}
	mark := "✓"
	if err != nil {
		mark = "✗"
	}
	pw.Freeze(mark + " " + msg)
}

func (pw *Writer) Print(msg string) {
	pw.PrintAt(msg, time.Time{})
}

func (pw *Writer) PrintAt(msg string, start time.Time) {
	if pw == nil || pw.silent {
		return
	}

	trimmed := trimTrailingSpace(msg)

	pw.mu.Lock()
	defer pw.mu.Unlock()

	pw.currentMsg = trimmed
	pw.currentStart = pw.resolvePrintStartLocked(start)
	if !pw.tty {
		line := pw.formatLineAt(trimmed, pw.currentStart)
		if line != pw.lastPlain {
			fmt.Fprintln(pw.w, line)
			pw.lastPlain = line
		}
		return
	}

	pw.startTickerLocked()
	pw.renderCurrentLocked()
}

func (pw *Writer) Freeze(msg string) {
	pw.FreezeAt(msg, time.Time{})
}

func (pw *Writer) FreezeAt(msg string, start time.Time) {
	if pw == nil || pw.silent {
		return
	}

	trimmed := trimTrailingSpace(msg)

	pw.mu.Lock()
	defer pw.mu.Unlock()

	if trimmed != "" {
		pw.currentMsg = trimmed
		pw.currentStart = pw.resolveFrozenStartLocked(start)
	}
	if pw.currentMsg == "" {
		return
	}

	if !pw.tty {
		line := pw.formatLineAt(pw.currentMsg, pw.currentStart)
		if line != pw.lastPlain {
			fmt.Fprintln(pw.w, line)
			pw.lastPlain = line
		}
		pw.currentMsg = ""
		pw.currentStart = time.Time{}
		pw.currentVisible = false
		return
	}

	pw.stopTickerLocked()
	fmt.Fprintf(pw.w, "\r%s%s\n", clearToEOL, pw.formatLineAt(pw.currentMsg, pw.currentStart))
	pw.currentMsg = ""
	pw.currentStart = time.Time{}
	pw.currentVisible = false
	pw.frozenLines++
}

func (pw *Writer) Keep() {
	pw.KeepAs("")
}

func (pw *Writer) KeepAs(msg string) {
	if pw == nil || pw.silent {
		return
	}

	pw.mu.Lock()
	defer pw.mu.Unlock()

	trimmed := trimTrailingSpace(msg)
	if trimmed != "" {
		pw.currentMsg = trimmed
		pw.currentStart = pw.resolveFrozenStartLocked(time.Time{})
	}
	if pw.currentMsg == "" {
		pw.stopTickerLocked()
		return
	}
	if !pw.tty {
		line := pw.formatLineAt(pw.currentMsg, pw.currentStart)
		if line != pw.lastPlain {
			fmt.Fprintln(pw.w, line)
			pw.lastPlain = line
		}
		pw.currentMsg = ""
		pw.currentStart = time.Time{}
		pw.currentVisible = false
		return
	}

	pw.stopTickerLocked()
	fmt.Fprintf(pw.w, "\r%s%s\n", clearToEOL, pw.formatLineAt(pw.currentMsg, pw.currentStart))
	pw.currentMsg = ""
	pw.currentStart = time.Time{}
	pw.currentVisible = false
	pw.frozenLines++
}

func (pw *Writer) Clear() {
	if pw == nil || pw.silent {
		return
	}

	pw.mu.Lock()
	defer pw.mu.Unlock()

	pw.stopTickerLocked()
	if !pw.tty {
		pw.currentMsg = ""
		pw.currentStart = time.Time{}
		pw.currentVisible = false
		return
	}

	linesRemaining := pw.frozenLines
	if pw.currentVisible {
		linesRemaining++
		fmt.Fprintf(pw.w, "\r%s", clearToEOL)
	} else if pw.frozenLines > 0 {
		fmt.Fprintf(pw.w, "%s\r%s", moveUpLine, clearToEOL)
		linesRemaining--
	} else {
		pw.currentMsg = ""
		pw.currentVisible = false
		return
	}

	for linesRemaining > 0 {
		fmt.Fprintf(pw.w, "%s\r%s", moveUpLine, clearToEOL)
		linesRemaining--
	}
	fmt.Fprint(pw.w, "\r")

	pw.currentMsg = ""
	pw.currentStart = time.Time{}
	pw.currentVisible = false
	pw.frozenLines = 0
}

func (pw *Writer) Close() {
	if pw == nil {
		return
	}
	pw.mu.Lock()
	defer pw.mu.Unlock()
	pw.stopTickerLocked()
}

func (pw *Writer) Elapsed() time.Duration {
	if pw == nil {
		return 0
	}
	return pw.now().Sub(pw.start)
}

func (pw *Writer) IsTTY() bool {
	if pw == nil {
		return false
	}
	return pw.tty
}

func FormatTimer(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d / time.Second)
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func FormatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int((d + 500*time.Millisecond) / time.Second)
	if seconds <= 0 {
		seconds = 0
	}
	return fmt.Sprintf("%ds", seconds)
}

func newWriter(w io.Writer, tty bool, width func() int, now func() time.Time, tickEvery time.Duration) *Writer {
	if now == nil {
		now = time.Now
	}
	if tickEvery <= 0 {
		tickEvery = time.Second
	}
	return &Writer{
		w:         w,
		tty:       tty,
		width:     width,
		start:     now(),
		now:       now,
		tickEvery: tickEvery,
	}
}

func (pw *Writer) tick() {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	if pw == nil || pw.silent || !pw.tty || !pw.currentVisible || pw.currentMsg == "" {
		return
	}
	pw.renderCurrentLocked()
}

func (pw *Writer) formatLine(msg string) string {
	return pw.formatLineAt(msg, pw.start)
}

func (pw *Writer) formatLineAt(msg string, start time.Time) string {
	line := fmt.Sprintf("%s %s", FormatTimer(pw.now().Sub(start)), trimTrailingSpace(msg))
	if !pw.tty {
		return line
	}
	return fitTTYLine(line, pw.terminalWidth())
}

func (pw *Writer) renderCurrentLocked() {
	if pw.currentMsg == "" {
		return
	}
	fmt.Fprintf(pw.w, "\r%s%s", clearToEOL, pw.formatLineAt(pw.currentMsg, pw.resolveFrozenStartLocked(time.Time{})))
	pw.currentVisible = true
}

func (pw *Writer) resolvePrintStartLocked(start time.Time) time.Time {
	if !start.IsZero() {
		return start
	}
	if !pw.start.IsZero() {
		return pw.start
	}
	return pw.now()
}

func (pw *Writer) resolveFrozenStartLocked(start time.Time) time.Time {
	if !start.IsZero() {
		return start
	}
	if !pw.currentStart.IsZero() {
		return pw.currentStart
	}
	if !pw.start.IsZero() {
		return pw.start
	}
	return pw.now()
}

func (pw *Writer) terminalWidth() int {
	if pw == nil || pw.width == nil {
		return 0
	}
	return pw.width()
}

func fitTTYLine(line string, width int) string {
	if width <= 1 {
		return line
	}

	runes := []rune(line)
	if len(runes) < width {
		return line
	}
	if width == 2 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

func (pw *Writer) startTickerLocked() {
	if !pw.tty || pw.tickEvery <= 0 || pw.tickerRunning || pw.currentMsg == "" {
		return
	}

	stop := make(chan struct{})
	pw.tickerStop = stop
	pw.tickerRunning = true
	tickEvery := pw.tickEvery

	go func() {
		ticker := time.NewTicker(tickEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pw.tick()
			case <-stop:
				return
			}
		}
	}()
}

func (pw *Writer) stopTickerLocked() {
	if !pw.tickerRunning {
		return
	}
	close(pw.tickerStop)
	pw.tickerRunning = false
	pw.tickerStop = nil
}

func (r *scopedReporter) Step(msg string) {
	if r == nil || r.writer == nil {
		return
	}
	r.writer.PrintAt(r.format(strings.TrimSpace(msg)), r.start)
}

func (r *scopedReporter) Stepf(format string, args ...any) {
	r.Step(fmt.Sprintf(format, args...))
}

func (r *scopedReporter) Child() Reporter {
	if r == nil {
		return Silence()
	}
	return &scopedReporter{
		writer: r.writer,
		prefix: r.prefix,
		label:  r.label,
		indent: r.indent + "  ",
		start:  r.start,
	}
}

func (r *scopedReporter) format(msg string) string {
	var b strings.Builder
	if r.prefix != "" {
		b.WriteString(r.prefix)
	}
	if r.indent != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(r.indent)
	}
	if msg != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(msg)
	}
	return trimTrailingSpace(b.String())
}

func buildPrefix(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return "building…"
	}
	return fmt.Sprintf("building %s…", trimmed)
}

func trimTrailingSpace(value string) string {
	return strings.TrimRight(value, " \t\r\n")
}
