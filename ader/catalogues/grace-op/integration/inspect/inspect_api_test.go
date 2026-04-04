package inspect_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInspect_API_LocalHolon(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.Inspect(&opv1.InspectRequest{Target: "gabriel-greeting-go"})
		if err != nil {
			t.Fatalf("api.Inspect: %v", err)
		}
		if len(resp.GetDocument().GetServices()) == 0 || len(resp.GetDocument().GetSequences()) == 0 {
			t.Fatalf("unexpected inspect document: %#v", resp.GetDocument())
		}
	})
}
