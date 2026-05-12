package cli

import (
	"os"
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

func TestApplyRunObservabilityActivationSources(t *testing.T) {
	for _, tc := range []struct {
		name       string
		opts       runObserveOptions
		envSet     bool
		envOPObs   string
		wantActive bool
		wantOPObs  string
	}{
		{
			name:       "observe flag activates without env",
			opts:       runObserveOptions{Observe: "logs,metrics"},
			wantActive: true,
			wantOPObs:  "logs,metrics",
		},
		{
			name:       "OP_OBS env activates without observe flag",
			envSet:     true,
			envOPObs:   "logs,events",
			wantActive: true,
			wantOPObs:  "logs,events",
		},
		{
			name:       "observe flag wins over OP_OBS env",
			opts:       runObserveOptions{Observe: "logs"},
			envSet:     true,
			envOPObs:   "metrics,events",
			wantActive: true,
			wantOPObs:  "logs",
		},
		{
			name: "no observe flag and no OP_OBS env is no-op",
		},
		{
			name:     "empty OP_OBS env is no-op",
			envSet:   true,
			envOPObs: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("OP_RUN_DIR", root)
			setOptionalEnv(t, "OP_OBS", tc.envOPObs, tc.envSet)
			cmd := exec.Command("true")

			uid, runRoot, err := applyRunObservability(cmd, "gabriel-greeting-go", tc.opts)
			if err != nil {
				t.Fatalf("applyRunObservability: %v", err)
			}

			if !tc.wantActive {
				if uid != "" || runRoot != "" {
					t.Fatalf("uid/runRoot = %q/%q, want no activation", uid, runRoot)
				}
				if cmd.Env != nil {
					t.Fatalf("cmd.Env = %#v, want no env injection", cmd.Env)
				}
				return
			}

			if uid == "" {
				t.Fatal("uid is empty")
			}
			if runRoot != root {
				t.Fatalf("runRoot = %q, want %q", runRoot, root)
			}
			if got := envValue(cmd.Env, "OP_INSTANCE_UID"); got != uid {
				t.Fatalf("OP_INSTANCE_UID = %q, want %q", got, uid)
			}
			if got := envValue(cmd.Env, "OP_RUN_DIR"); got != root {
				t.Fatalf("OP_RUN_DIR = %q, want %q", got, root)
			}
			if got := envValue(cmd.Env, "OP_OBS"); got != tc.wantOPObs {
				t.Fatalf("OP_OBS = %q, want %q", got, tc.wantOPObs)
			}
		})
	}
}

func setOptionalEnv(t *testing.T, key, value string, set bool) {
	t.Helper()
	oldValue, oldSet := os.LookupEnv(key)
	if set {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("setenv %s: %v", key, err)
		}
	} else if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if oldSet {
			_ = os.Setenv(key, oldValue)
		} else {
			_ = os.Unsetenv(key)
		}
	})
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
