package invoke_test

import (
	"context"
	"strings"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInvoke_RPC_GoVersion(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Invoke(ctx, &opv1.InvokeRequest{
		Holon: "go",
		Args:  []string{"version"},
	})
	if err != nil {
		t.Fatalf("rpc Invoke: %v", err)
	}
	if resp.GetExitCode() != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", resp.GetExitCode(), resp.GetStderr())
	}
	if !strings.Contains(resp.GetStdout(), "go version") {
		t.Fatalf("stdout = %q, want to contain go version", resp.GetStdout())
	}
}
