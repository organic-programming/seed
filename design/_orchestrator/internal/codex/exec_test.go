package codex

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/organic-programming/codex-orchestrator/internal/cli"
)

func TestExecuteWithWritersCreatesTimestampedLogs(t *testing.T) {
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}

	scriptPath := filepath.Join(binDir, "codex")
	script := `#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
	if [ "$1" = "-o" ]; then
		shift
		out="$1"
	fi
	shift
done
printf '%s\n' '{"type":"thread.started","thread_id":"thread-123"}'
printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":5,"cached_input_tokens":2,"output_tokens":3}}'
printf '%s\n' 'stderr line' >&2
printf 'final message\n' > "$out"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	taskFile := filepath.Join(tempDir, "TASK01.md")
	if err := os.WriteFile(taskFile, []byte("# task\n"), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	cfg := cli.Config{
		Model: "gpt-5.4",
		Root:  tempDir,
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	result, err := executeWithWriters(cfg, "implement task", taskFile, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("executeWithWriters returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result")
	}
	if result.ThreadID != "thread-123" {
		t.Fatalf("unexpected thread id: %q", result.ThreadID)
	}
	if result.Tokens.InputTokens != 5 || result.Tokens.CachedInputTokens != 2 || result.Tokens.OutputTokens != 3 {
		t.Fatalf("unexpected token totals: %+v", result.Tokens)
	}
	if result.Output != "final message\n" {
		t.Fatalf("unexpected result output: %q", result.Output)
	}

	pattern := regexp.MustCompile(`^\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_\d{3} `)

	jsonl, err := os.ReadFile(taskFile + ".jsonl")
	if err != nil {
		t.Fatalf("read jsonl log: %v", err)
	}
	stderrLog, err := os.ReadFile(taskFile + ".stderr.log")
	if err != nil {
		t.Fatalf("read stderr log: %v", err)
	}
	resultFile, err := os.ReadFile(taskFile + ".result.md")
	if err != nil {
		t.Fatalf("read result file: %v", err)
	}

	if !pattern.Match(jsonl) {
		t.Fatalf("jsonl log missing timestamp prefix: %q", string(jsonl))
	}
	if !pattern.Match(stderrLog) {
		t.Fatalf("stderr log missing timestamp prefix: %q", string(stderrLog))
	}
	if !pattern.Match(stdout.Bytes()) {
		t.Fatalf("stdout mirror missing timestamp prefix: %q", stdout.String())
	}
	if !pattern.Match(stderr.Bytes()) {
		t.Fatalf("stderr mirror missing timestamp prefix: %q", stderr.String())
	}
	if string(resultFile) != "final message\n" {
		t.Fatalf("unexpected result file contents: %q", string(resultFile))
	}
}
