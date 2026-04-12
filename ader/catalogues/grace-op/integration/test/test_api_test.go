package test_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestTest_API_RepresentativeHolon(t *testing.T) {
	var target string
	for _, spec := range integration.NativeTestHolons(t) {
		if spec.Slug == "matt-calculator-go" && integration.SupportsOPTest(spec) {
			target = spec.Slug
			break
		}
	}
	if target == "" {
		for _, spec := range integration.NativeTestHolons(t) {
			if spec.Slug != "gabriel-greeting-go" && integration.SupportsOPTest(spec) {
				target = spec.Slug
				break
			}
		}
	}
	if target == "" {
		t.Skip("no suitable non-go holon available for op test")
	}

	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.Test(&opv1.LifecycleRequest{Target: target})
		if err != nil {
			t.Fatalf("api.Test(%s): %v", target, err)
		}
		if resp.GetReport().GetOperation() != "test" {
			t.Fatalf("operation = %q, want test", resp.GetReport().GetOperation())
		}
	})
}
