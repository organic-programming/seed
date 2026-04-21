//go:build e2e

package invoke_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInvoke_CLI_CleanFlag(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "gabriel-greeting-go")

	result := sb.RunOP(t, "invoke", "--clean", "gabriel-greeting-go", "SayHello", `{"name":"World","lang_code":"en"}`)
	integration.RequireSuccess(t, result)
	payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
	if payload["greeting"] == "" {
		t.Fatalf("empty invoke payload after clean: %#v", payload)
	}
}

func TestInvoke_CLI_NoBuildDoesNotBuild(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.RemoveArtifactFor(t, sb, "gabriel-greeting-go")
	artifactPath := integration.ArtifactPathFor(t, sb, "gabriel-greeting-go")

	result := sb.RunOP(t, "invoke", "gabriel-greeting-go", "SayHello", "--no-build", `{"name":"World","lang_code":"en"}`)
	if result.TimedOut {
		t.Fatalf("--no-build timed out\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr)
	}
	integration.RequirePathMissing(t, artifactPath)
}

func TestInvoke_CLI_CleanNoBuildConflict(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "invoke", "--clean", "--no-build", "gabriel-greeting-go", "SayHello", `{"name":"World","lang_code":"en"}`)
	integration.RequireFailure(t, result)
	integration.RequireContains(t, result.Stderr, "--clean cannot be combined with --no-build")
}
