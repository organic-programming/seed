//go:build windows

package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/clem-ader/internal/testrepo"
)

func TestWindowsAderDescendantHelper(t *testing.T) {
	role := os.Getenv("ADER_NATIVE_PROCESS_ROLE")
	if role == "" {
		return
	}
	pidFile := os.Getenv("ADER_NATIVE_PROCESS_PID")
	if role == "leaf" {
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(2)
		}
		for {
			time.Sleep(time.Second)
		}
	}
	executable, _ := os.Executable()
	child := exec.Command(executable, "-test.run=^TestWindowsAderDescendantHelper$")
	child.Env = append(os.Environ(), "ADER_NATIVE_PROCESS_ROLE=leaf")
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(3)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if _, err := os.Stat(pidFile); err == nil {
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(4)
}

func TestRunStepCommandTerminatesBackgroundProcesses(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(root, "child.pid")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	code, err := runStepCommand(ctx, StepSpec{
		ID: "native-background", Workdir: root, Script: executable,
		Args: []string{"-test.run=^TestWindowsAderDescendantHelper$"},
	}, append(os.Environ(), "ADER_NATIVE_PROCESS_ROLE=parent", "ADER_NATIVE_PROCESS_PID="+pidFile), filepath.Join(root, "step.log"), nil)
	if err != nil || code != 0 {
		t.Fatalf("native background step: exit=%d err=%v", code, err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, 2*time.Second, func() bool { return !processRunning(pid) }, "background Windows descendant survived its step")
	if !processRunning(os.Getpid()) {
		t.Fatal("live test process was treated as stale")
	}
}

func TestWindowsCommittedSnapshotStopsProducerOnExtractError(t *testing.T) {
	repo := testrepo.Create(t)
	destination := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(destination, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := snapshotCommitted(ctx, repo, destination); err == nil {
		t.Fatal("expected extraction failure")
	}
	if ctx.Err() != nil {
		t.Fatal("archive producer remained blocked after extraction failed")
	}
}
