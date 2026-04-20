//go:build e2e

package do_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestDo_API_DryRun(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.RunSequence(&opv1.RunSequenceRequest{
			Holon:    "gabriel-greeting-go",
			Sequence: "multilingual-greeting",
			Params: map[string]string{
				"name":      "Alice",
				"lang_code": "fr",
			},
			DryRun: true,
		})
		if err != nil {
			t.Fatalf("api.RunSequence: %v", err)
		}
		if len(resp.GetResult().GetSteps()) < 2 {
			t.Fatalf("expected at least two sequence steps, got %#v", resp.GetResult())
		}
		if !strings.Contains(resp.GetResult().GetSteps()[0].GetCommand(), "ListLanguages") {
			t.Fatalf("unexpected first step: %#v", resp.GetResult().GetSteps()[0])
		}
	})
}
