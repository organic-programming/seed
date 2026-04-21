//go:build e2e

package do_test

import (
	"context"
	"strings"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestDo_RPC_DryRun(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.RunSequence(ctx, &opv1.RunSequenceRequest{
		Holon:    "gabriel-greeting-go",
		Sequence: "multilingual-greeting",
		Params: map[string]string{
			"name":      "Alice",
			"lang_code": "fr",
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("rpc RunSequence: %v", err)
	}
	if len(resp.GetResult().GetSteps()) < 2 {
		t.Fatalf("expected at least two sequence steps, got %#v", resp.GetResult())
	}
	if !strings.Contains(resp.GetResult().GetSteps()[0].GetCommand(), "ListLanguages") {
		t.Fatalf("unexpected first step: %#v", resp.GetResult().GetSteps()[0])
	}
}
