package invoke_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

func TestInvoke_API_GoVersion(t *testing.T) {
	resp, err := api.Invoke(&opv1.InvokeRequest{
		Holon: "go",
		Args:  []string{"version"},
	})
	if err != nil {
		t.Fatalf("api.Invoke: %v", err)
	}
	if resp.GetExitCode() != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", resp.GetExitCode(), resp.GetStderr())
	}
	if !strings.Contains(resp.GetStdout(), "go version") {
		t.Fatalf("stdout = %q, want to contain go version", resp.GetStdout())
	}
}
