package runner

import (
	"context"

	"github.com/organic-programming/james-loops/internal/profile"
)

type geminiRunner struct {
	profile profile.Profile
}

func (r geminiRunner) Run(ctx context.Context, repoRoot, prompt string) (int, []byte, []byte, error) {
	args := append([]string{}, r.profile.ExtraArgs...)
	args = append(args, "-p", prompt)
	if r.profile.Model != "" {
		args = append(args, "-m", r.profile.Model)
	}
	return execRunnerCommand(ctx, repoRoot, commandSpec{Name: "gemini", Args: args})
}

func (r geminiRunner) IsQuotaIssue(exitCode int, stdout, stderr []byte) bool {
	return isQuotaIssueWithPhrases(exitCode, stdout, stderr, r.profile.QuotaPhrases)
}

func (r geminiRunner) ProbeSaysReady(exitCode int, stdout, stderr []byte) bool {
	return probeSaysReady(exitCode, stdout, stderr)
}

func (r geminiRunner) QuotaProbePrompt() string {
	return effectiveQuotaPrompt(r.profile.QuotaProbePrompt)
}
