package logging

import (
	"bytes"
	"os"
	"regexp"
	"testing"
)

func TestTimestampNowFormat(t *testing.T) {
	t.Parallel()

	pattern := regexp.MustCompile(`^\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_\d{3}$`)
	if !pattern.MatchString(TimestampNow()) {
		t.Fatalf("unexpected timestamp format: %q", TimestampNow())
	}
}

func TestTeeWriterPrefixesTerminalAndLog(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "tee-*.log")
	if err != nil {
		t.Fatalf("create temp log: %v", err)
	}

	var terminal bytes.Buffer
	writer := &TeeWriter{
		Terminal: &terminal,
		LogFile:  file,
	}

	writer.WriteLine("hello world")

	if err := file.Close(); err != nil {
		t.Fatalf("close log file: %v", err)
	}

	logContent, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	pattern := regexp.MustCompile(`^\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_\d{3} hello world\n$`)
	if !pattern.Match(terminal.Bytes()) {
		t.Fatalf("terminal output missing timestamp prefix: %q", terminal.String())
	}
	if !pattern.Match(logContent) {
		t.Fatalf("log output missing timestamp prefix: %q", string(logContent))
	}
	if terminal.String() != string(logContent) {
		t.Fatalf("terminal and log output differ: %q != %q", terminal.String(), string(logContent))
	}
}
