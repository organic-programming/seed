package runner

import (
	"context"

	"github.com/organic-programming/james-loops/internal/profile"
)

type ollamaRunner struct {
	profile profile.Profile
}

func (r ollamaRunner) Run(ctx context.Context, repoRoot, prompt string) (int, []byte, []byte, error) {
	args := []string{"run", r.profile.Model}
	args = append(args, r.profile.ExtraArgs...)
	args = append(args, prompt)
	return execRunnerCommand(ctx, repoRoot, commandSpec{Name: "ollama", Args: args})
}

func (r ollamaRunner) IsQuotaIssue(exitCode int, stdout, stderr []byte) bool {
	return false
}

func (r ollamaRunner) ProbeSaysReady(exitCode int, stdout, stderr []byte) bool {
	return probeSaysReady(exitCode, stdout, stderr)
}

func (r ollamaRunner) QuotaProbePrompt() string {
	return effectiveQuotaPrompt(r.profile.QuotaProbePrompt)
}
