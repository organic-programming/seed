package engine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type GateRunner interface {
	Run(ctx context.Context, repoRoot string, gate Gate) (passed bool, reportPath string, err error)
}

type shellGateRunner struct{}

func (shellGateRunner) Run(ctx context.Context, repoRoot string, gate Gate) (passed bool, reportPath string, err error) {
	return runGate(ctx, repoRoot, gate)
}

func runGate(ctx context.Context, repoRoot string, gate Gate) (passed bool, reportPath string, err error) {
	log.Printf("running gate command: %s", gate.Command)
	cmd := exec.CommandContext(ctx, "sh", "-c", gate.Command)
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if ok := errorAs(runErr, &exitErr); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return false, "", fmt.Errorf("run gate %q: %w", gate.Command, runErr)
		}
	}

	reportPath = parseReportPath(stdout.Bytes())
	expect := strings.ToUpper(strings.TrimSpace(gate.Expect))
	switch expect {
	case "", "PASS":
		passed = exitCode == 0
	case "FAIL":
		passed = exitCode != 0
	default:
		return false, reportPath, fmt.Errorf("unsupported gate expectation %q", gate.Expect)
	}
	return passed, reportPath, nil
}

func parseReportPath(stdout []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "report:") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "report:"))
	}
	return ""
}
