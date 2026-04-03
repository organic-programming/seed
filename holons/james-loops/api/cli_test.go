package api

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestRunCLIVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunCLI([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCLI(version) = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); !strings.HasPrefix(got, "james-loops ") {
		t.Fatalf("version output = %q, want prefix %q", got, "james-loops ")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCommandShape(t *testing.T) {
	root := newRootCommand(io.Discard, io.Discard)
	run := findCommand(root, "run")
	if run == nil {
		t.Fatal("run command missing")
	}
	if run.Use != "run" {
		t.Fatalf("run use = %q, want %q", run.Use, "run")
	}
	if run.Flags().Lookup("root") == nil ||
		run.Flags().Lookup("dry-run") == nil ||
		run.Flags().Lookup("max-retries") == nil ||
		run.Flags().Lookup("coder-profile") == nil ||
		run.Flags().Lookup("evaluator-profile") == nil {
		t.Fatal("run flags missing")
	}
}

func TestEnqueueCommandShape(t *testing.T) {
	root := newRootCommand(io.Discard, io.Discard)
	cmd := findCommand(root, "enqueue")
	if cmd == nil {
		t.Fatal("enqueue command missing")
	}
	if cmd.Use != "enqueue <program-dir>" {
		t.Fatalf("enqueue use = %q, want %q", cmd.Use, "enqueue <program-dir>")
	}
	if cmd.Flags().Lookup("from-cookbook") == nil || cmd.Flags().Lookup("root") == nil {
		t.Fatal("enqueue flags missing")
	}
}

func TestDropAndLogCommandShape(t *testing.T) {
	root := newRootCommand(io.Discard, io.Discard)
	drop := findCommand(root, "drop")
	if drop == nil {
		t.Fatal("drop command missing")
	}
	if drop.Use != "drop <slot>" {
		t.Fatalf("drop use = %q, want %q", drop.Use, "drop <slot>")
	}
	if drop.Flags().Lookup("deferred") == nil || drop.Flags().Lookup("root") == nil {
		t.Fatal("drop flags missing")
	}

	logCmd := findCommand(root, "log")
	if logCmd == nil {
		t.Fatal("log command missing")
	}
	if logCmd.Use != "log <step-id>" {
		t.Fatalf("log use = %q, want %q", logCmd.Use, "log <step-id>")
	}
	if logCmd.Flags().Lookup("root") == nil {
		t.Fatal("log flags missing")
	}
}

func TestProfileCommandShape(t *testing.T) {
	root := newRootCommand(io.Discard, io.Discard)
	cmd := findCommand(root, "profile")
	if cmd == nil {
		t.Fatal("profile command missing")
	}
	for _, name := range []string{"list", "show", "validate"} {
		if findCommand(cmd, name) == nil {
			t.Fatalf("profile subcommand %q missing", name)
		}
	}
}
