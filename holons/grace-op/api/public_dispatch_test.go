package api_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

func TestInvokeRunsGoVersion(t *testing.T) {
	resp, err := api.Invoke(&opv1.InvokeRequest{
		Holon: "go",
		Args:  []string{"version"},
	})
	if err != nil {
		t.Fatalf("Invoke error = %v", err)
	}
	if got := resp.GetExitCode(); got != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", got, resp.GetStderr())
	}
	if !strings.Contains(resp.GetStdout(), "go version") {
		t.Fatalf("stdout = %q, want to contain %q", resp.GetStdout(), "go version")
	}
}
