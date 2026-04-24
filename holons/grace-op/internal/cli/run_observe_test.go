package cli

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInjectObservabilityEnvUsesRegistryRoot(t *testing.T) {
	root := t.TempDir()
	uid := "uid123"
	cmd := exec.Command("true")

	injectObservabilityEnv(cmd, uid, root, runObserveOptions{Observe: "logs,metrics"})

	if got := envValue(cmd.Env, "OP_INSTANCE_UID"); got != uid {
		t.Fatalf("OP_INSTANCE_UID = %q, want %q", got, uid)
	}
	if got := envValue(cmd.Env, "OP_RUN_DIR"); got != root {
		t.Fatalf("OP_RUN_DIR = %q, want registry root %q", got, root)
	}
	if got := envValue(cmd.Env, "OP_RUN_DIR"); got == filepath.Join(root, "gabriel-greeting-go", uid) {
		t.Fatalf("OP_RUN_DIR was injected as per-instance path %q", got)
	}
	if got := envValue(cmd.Env, "OP_OBS"); got != "logs,metrics" {
		t.Fatalf("OP_OBS = %q, want logs,metrics", got)
	}
}
