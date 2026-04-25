package cli

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/go-holons/pkg/observability"
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

func TestApplyRunObservabilityRejectsOtelInV1(t *testing.T) {
	for _, opts := range []runObserveOptions{
		{OTel: "otel-collector:4317"},
		{Observe: "logs,otel"},
	} {
		cmd := exec.Command("true")
		_, _, err := applyRunObservability(cmd, "gabriel-greeting-go", opts)
		if err == nil {
			t.Fatalf("applyRunObservability(%+v) succeeded, want otel rejection", opts)
		}
		if !strings.Contains(err.Error(), "reserved for observability v2") {
			t.Fatalf("error = %q, want v2 reservation", err.Error())
		}
	}
}

func TestReadObservedInstanceWaitsForRequestedSlug(t *testing.T) {
	root := t.TempDir()
	uid := "uid-1"

	childDir, err := observability.InstanceRunDir(root, "gabriel-greeting-swift", uid)
	if err != nil {
		t.Fatalf("child run dir: %v", err)
	}
	if err := observability.WriteMetaJSON(childDir, observability.MetaJSON{
		Slug:      "gabriel-greeting-swift",
		UID:       uid,
		PID:       1,
		StartedAt: time.Now(),
		Transport: "stdio",
		Address:   "stdio://",
	}); err != nil {
		t.Fatalf("write child meta: %v", err)
	}

	if inst, ok := readObservedInstance("gabriel-greeting-app-flutter", uid, root); ok {
		t.Fatalf("readObservedInstance matched child %+v for requested root slug", inst.meta)
	}

	rootDir, err := observability.InstanceRunDir(root, "gabriel-greeting-app-flutter", uid)
	if err != nil {
		t.Fatalf("root run dir: %v", err)
	}
	if err := observability.WriteMetaJSON(rootDir, observability.MetaJSON{
		Slug:      "gabriel-greeting-app-flutter",
		UID:       uid,
		PID:       1,
		StartedAt: time.Now(),
		Transport: "tcp",
		Address:   "tcp://127.0.0.1:1234",
	}); err != nil {
		t.Fatalf("write root meta: %v", err)
	}

	inst, ok := readObservedInstance("gabriel-greeting-app-flutter", uid, root)
	if !ok {
		t.Fatal("readObservedInstance did not match requested root slug")
	}
	if inst.meta.Slug != "gabriel-greeting-app-flutter" {
		t.Fatalf("slug = %q", inst.meta.Slug)
	}
}
