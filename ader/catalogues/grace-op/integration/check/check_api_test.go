package check_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestCheck_API_GabrielGreetingGo(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.Check(&opv1.LifecycleRequest{Target: "gabriel-greeting-go"})
		if err != nil {
			t.Fatalf("api.Check: %v", err)
		}
		if resp.GetReport().GetOperation() != "check" {
			t.Fatalf("operation = %q, want check", resp.GetReport().GetOperation())
		}
	})
}
