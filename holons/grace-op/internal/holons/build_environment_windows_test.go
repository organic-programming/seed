package holons

import (
	"testing"

	"github.com/organic-programming/grace-op/internal/buildenv"
)

func TestBuildEnvironmentDoesNotOverrideRecipeContext(t *testing.T) {
	ctx := BuildContext{
		Target: "windows", Mode: "release",
		Environment:      []string{"OP_BUILD_TARGET=macos", "OP_BUILD_MODE=debug", "PRIVATE_TOOL=available"},
		SDKPrebuiltPaths: map[string]string{"go": "sdk-go"},
	}
	env := buildExecutionEnv(nil, ctx)
	for key, want := range map[string]string{"OP_BUILD_TARGET": "windows", "OP_BUILD_MODE": "release", "PRIVATE_TOOL": "available", "OP_SDK_GO_PATH": "sdk-go"} {
		if got := buildenv.Value(env, key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}
