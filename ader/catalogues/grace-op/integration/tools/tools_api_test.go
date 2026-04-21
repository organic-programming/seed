//go:build e2e

package tools_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestTools_API_ReturnsOpenAIToolDefinitions(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.Tools(&opv1.ToolsRequest{Target: "gabriel-greeting-go", Format: "openai"})
		if err != nil {
			t.Fatalf("api.Tools: %v", err)
		}
		if resp.GetFormat() != "openai" || !strings.Contains(string(resp.GetPayload()), `"function"`) {
			t.Fatalf("unexpected tools payload: format=%q payload=%s", resp.GetFormat(), string(resp.GetPayload()))
		}
	})
}
