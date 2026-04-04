package env_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestEnv_API_InitializesDirectoriesAndShell(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.Env(&opv1.EnvRequest{Init: true, Shell: true})
		if err != nil {
			t.Fatalf("api.Env: %v", err)
		}
		if resp.GetOppath() == "" || resp.GetOpbin() == "" || resp.GetRoot() == "" {
			t.Fatalf("unexpected env response: %#v", resp)
		}
		if !strings.Contains(resp.GetShell(), "export OPPATH") {
			t.Fatalf("shell snippet = %q, want export OPPATH", resp.GetShell())
		}
	})
}
