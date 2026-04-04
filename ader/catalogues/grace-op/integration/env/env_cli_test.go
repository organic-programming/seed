package env_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestEnv_CLI_Text(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "env")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "OPPATH=")
	integration.RequireContains(t, result.Stdout, "OPBIN=")
	integration.RequireContains(t, result.Stdout, "ROOT=")
}

func TestEnv_CLI_JSON(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "--format", "json", "env")
	integration.RequireSuccess(t, result)
	payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
	if payload["oppath"] == "" || payload["opbin"] == "" || payload["root"] == "" {
		t.Fatalf("unexpected env payload: %#v", payload)
	}
}

func TestEnv_CLI_InitAndShell(t *testing.T) {
	sb := integration.NewSandbox(t)
	home := t.TempDir()
	result := sb.RunOPWithOptions(t, integration.RunOptions{
		Env: []string{
			"HOME=" + home,
			"OPPATH=" + filepath.Join(home, ".op"),
			"OPBIN=" + filepath.Join(home, ".op", "bin"),
		},
	}, "env", "--init", "--shell")
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "export OPPATH=")
	if _, err := os.Stat(filepath.Join(home, ".op")); err != nil {
		t.Fatalf("expected initialized OPPATH: %v", err)
	}
}
