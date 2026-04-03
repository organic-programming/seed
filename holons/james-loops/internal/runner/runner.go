package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/organic-programming/james-loops/internal/profile"
)

const defaultQuotaProbePrompt = "Are you ready? Respond y / n only."

var defaultQuotaPhrases = []string{
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

type commandSpec struct {
	Name string
	Args []string
}

type commandExecutor func(ctx context.Context, repoRoot string, spec commandSpec) (exitCode int, stdout, stderr []byte, err error)

var execRunnerCommand commandExecutor = defaultExecRunnerCommand

// AIRunner abstracts any AI CLI that can process a prompt.
type AIRunner interface {
	Run(ctx context.Context, repoRoot, prompt string) (exitCode int, stdout, stderr []byte, err error)
	IsQuotaIssue(exitCode int, stdout, stderr []byte) bool
	ProbeSaysReady(exitCode int, stdout, stderr []byte) bool
	QuotaProbePrompt() string
}

// New creates the AIRunner for the given profile.
// Returns an error for unknown drivers.
func New(p profile.Profile) (AIRunner, error) {
	switch p.Driver {
	case profile.DriverCodex:
		return codexRunner{profile: p}, nil
	case profile.DriverGemini:
		return geminiRunner{profile: p}, nil
	case profile.DriverOllama:
		return ollamaRunner{profile: p}, nil
	default:
		return nil, fmt.Errorf("unknown driver %q", p.Driver)
	}
}

func defaultExecRunnerCommand(ctx context.Context, repoRoot string, spec commandSpec) (int, []byte, []byte, error) {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	if strings.TrimSpace(repoRoot) != "" {
		cmd.Dir = repoRoot
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.Bytes(), stderr.Bytes(), nil
	}
	if strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "signal: killed") {
		return -1, stdout.Bytes(), stderr.Bytes(), err
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), stdout.Bytes(), stderr.Bytes(), nil
	}
	return -1, stdout.Bytes(), stderr.Bytes(), err
}

func normalizedOutput(stdout []byte, stderr []byte) string {
	return strings.ToLower(strings.TrimSpace(string(stdout) + "\n" + string(stderr)))
}

func isQuotaIssueWithPhrases(exitCode int, stdout []byte, stderr []byte, phrases []string) bool {
	if exitCode == 0 {
		return false
	}
	text := normalizedOutput(stdout, stderr)
	for _, phrase := range effectiveQuotaPhrases(phrases) {
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(phrase))) {
			return true
		}
	}
	return false
}

func probeSaysReady(exitCode int, stdout []byte, stderr []byte) bool {
	if exitCode != 0 {
		return false
	}
	text := strings.TrimSpace(normalizedOutput(stdout, stderr))
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

func effectiveQuotaPhrases(phrases []string) []string {
	if len(phrases) == 0 {
		return defaultQuotaPhrases
	}
	return phrases
}

func effectiveQuotaPrompt(prompt string) string {
	if strings.TrimSpace(prompt) == "" {
		return defaultQuotaProbePrompt
	}
	return prompt
}

func hasFlag(args []string, name string) bool {
	for i := range args {
		if args[i] == name {
			return true
		}
	}
	return false
}

// WithGeminiJSONOutput appends `--output-format json` when needed.
func WithGeminiJSONOutput(p profile.Profile) profile.Profile {
	if p.Driver != profile.DriverGemini || hasFlag(p.ExtraArgs, "--output-format") {
		return p
	}
	copyProfile := p
	copyProfile.ExtraArgs = append(append([]string{}, p.ExtraArgs...), "--output-format", "json")
	return copyProfile
}
