//go:build e2e

package tools_test

import (
	"context"
	"strings"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestTools_RPC_ReturnsOpenAIToolDefinitions(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Tools(ctx, &opv1.ToolsRequest{Target: "gabriel-greeting-go", Format: "openai"})
	if err != nil {
		t.Fatalf("rpc Tools: %v", err)
	}
	if resp.GetFormat() != "openai" || !strings.Contains(string(resp.GetPayload()), `"function"`) {
		t.Fatalf("unexpected tools payload: format=%q payload=%s", resp.GetFormat(), string(resp.GetPayload()))
	}
}
