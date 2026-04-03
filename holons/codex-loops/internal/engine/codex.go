package engine

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const quotaProbePrompt = "Are you ready? Respond y / n only."

var quotaIssuePhrases = []string{
	"quota",
	"rate limit",
	"usage limit",
	"credit balance",
	"credits",
	"billing hard limit",
	"hard limit",
	"too many requests",
	"429",
	"plan limit",
}

type CodexRunner interface {
	Run(ctx context.Context, repoRoot, prompt string) (exitCode int, stdout, stderr []byte, err error)
}

type shellCodexRunner struct{}

func (shellCodexRunner) Run(ctx context.Context, repoRoot, prompt string) (int, []byte, []byte, error) {
	log.Printf("running command: codex exec --full-auto -a never -C %s <prompt>", repoRoot)
	cmd := exec.CommandContext(ctx, "codex", "exec", "--full-auto", "-a", "never", "-C", repoRoot, prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.Bytes(), stderr.Bytes(), nil
	}
	var exitErr *exec.ExitError
	if strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "signal: killed") {
		return -1, stdout.Bytes(), stderr.Bytes(), err
	}
	if ok := errorAs(err, &exitErr); ok {
		return exitErr.ExitCode(), stdout.Bytes(), stderr.Bytes(), nil
	}
	return -1, stdout.Bytes(), stderr.Bytes(), err
}

func runCodex(ctx context.Context, repoRoot string, briefPath string, retryContext string) (exitCode int, stdout []byte, stderr []byte, err error) {
	brief, err := os.ReadFile(briefPath)
	if err != nil {
		return -1, nil, nil, fmt.Errorf("read brief %s: %w", briefPath, err)
	}
	prompt := strings.TrimSpace(string(brief))
	if strings.TrimSpace(retryContext) != "" {
		prompt = prompt + "\n\n--- PREVIOUS ATTEMPT FAILED ---\n" + strings.TrimSpace(retryContext)
	}
	return shellCodexRunner{}.Run(ctx, repoRoot, prompt)
}

func errorAs(err error, target interface{}) bool {
	switch t := target.(type) {
	case **exec.ExitError:
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return false
		}
		*t = exitErr
		return true
	default:
		return false
	}
}

func isQuotaIssue(exitCode int, stdout []byte, stderr []byte) bool {
	if exitCode == 0 {
		return false
	}
	text := normalizedCodexOutput(stdout, stderr)
	for _, phrase := range quotaIssuePhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func probeSaysReady(exitCode int, stdout []byte, stderr []byte) bool {
	if exitCode != 0 {
		return false
	}
	text := strings.TrimSpace(normalizedCodexOutput(stdout, stderr))
	if text == "" {
		return false
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	token := strings.Trim(fields[0], ".,:;!?")
	return token == "y" || token == "yes"
}

func normalizedCodexOutput(stdout []byte, stderr []byte) string {
	return strings.ToLower(strings.TrimSpace(string(stdout) + "\n" + string(stderr)))
}
