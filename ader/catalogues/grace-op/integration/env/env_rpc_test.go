package env_test

import (
	"context"
	"strings"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestEnv_RPC_InitializesDirectoriesAndShell(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Env(ctx, &opv1.EnvRequest{Init: true, Shell: true})
	if err != nil {
		t.Fatalf("rpc Env: %v", err)
	}
	if resp.GetOppath() == "" || resp.GetOpbin() == "" || resp.GetRoot() == "" {
		t.Fatalf("unexpected env response: %#v", resp)
	}
	if !strings.Contains(resp.GetShell(), "export OPPATH") {
		t.Fatalf("shell snippet = %q, want export OPPATH", resp.GetShell())
	}
}
