package runner

import (
	"context"
	"github.com/organic-programming/james-loops/internal/profile"
	"log"
)

var codexDefaultArgs = []string{"--full-auto", "-a", "never"}

type codexRunner struct {
	profile profile.Profile
}

func (r codexRunner) Run(ctx context.Context, repoRoot, prompt string) (int, []byte, []byte, error) {
	args := []string{"exec"}
	args = append(args, r.effectiveArgs()...)
	if r.profile.Model != "" {
		args = append(args, "--model", r.profile.Model)
	}
	args = append(args, "-C", repoRoot, prompt)
	log.Printf("running command: codex %v", args)
	return execRunnerCommand(ctx, repoRoot, commandSpec{Name: "codex", Args: args})
}

func (r codexRunner) IsQuotaIssue(exitCode int, stdout, stderr []byte) bool {
	return isQuotaIssueWithPhrases(exitCode, stdout, stderr, r.profile.QuotaPhrases)
}

func (r codexRunner) ProbeSaysReady(exitCode int, stdout, stderr []byte) bool {
	return probeSaysReady(exitCode, stdout, stderr)
}

func (r codexRunner) QuotaProbePrompt() string {
	return effectiveQuotaPrompt(r.profile.QuotaProbePrompt)
}

func (r codexRunner) effectiveArgs() []string {
	if len(r.profile.ExtraArgs) == 0 {
		return append([]string{}, codexDefaultArgs...)
	}
	return append([]string{}, r.profile.ExtraArgs...)
}
