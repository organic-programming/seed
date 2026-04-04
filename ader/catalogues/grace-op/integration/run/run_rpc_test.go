package run_test

import (
	"context"
	"strings"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestRun_RPC_NoBuildFailsWhenArtifactMissing(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.RemoveArtifactFor(t, sb, "gabriel-greeting-go")

	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Run(ctx, &opv1.RunRequest{Holon: "gabriel-greeting-go", NoBuild: true})
	if err == nil {
		t.Fatal("expected rpc Run to fail when artifact is missing and no_build is set")
	}
	if !strings.Contains(err.Error(), "artifact missing") {
		t.Fatalf("error = %v, want artifact missing", err)
	}
}
