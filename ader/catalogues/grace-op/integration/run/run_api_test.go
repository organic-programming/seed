//go:build e2e

package run_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestRun_API_NoBuildFailsWhenArtifactMissing(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.RemoveArtifactFor(t, sb, "gabriel-greeting-go")

	integration.WithSandboxEnv(t, sb, func() {
		_, err := api.Run(&opv1.RunRequest{Holon: "gabriel-greeting-go", NoBuild: true})
		if err == nil {
			t.Fatal("expected api.Run to fail when artifact is missing and no_build is set")
		}
		if !strings.Contains(err.Error(), "artifact missing") {
			t.Fatalf("error = %v, want artifact missing", err)
		}
	})
}
