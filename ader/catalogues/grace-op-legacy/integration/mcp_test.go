// MCP tests drive the real stdio MCP bridge with JSON-RPC and verify tool and
// prompt exposure for one or more hello-world holons.
package integration

import (
	"testing"
)

func TestMCP_LocalHolonHandshakeAndTools(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	responses, result := mcpConversation(t, sb, []string{"gabriel-greeting-go"}, []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "prompts/list",
			"params":  map[string]any{},
		},
	})
	requireSuccess(t, result)
	if len(responses) != 3 {
		t.Fatalf("responses = %d, want 3", len(responses))
	}

	toolsResult, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list result = %#v", responses[1]["result"])
	}
	tools, ok := toolsResult["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools/list tools = %#v", toolsResult["tools"])
	}

	foundSequence := false
	for _, entry := range tools {
		tool := entry.(map[string]any)
		if tool["name"] == "gabriel-greeting-go.sequence.multilingual-greeting" {
			foundSequence = true
			break
		}
	}
	if !foundSequence {
		t.Fatalf("sequence tool missing from tools/list: %#v", tools)
	}

	promptsResult, ok := responses[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("prompts/list result = %#v", responses[2]["result"])
	}
	prompts, ok := promptsResult["prompts"].([]any)
	if !ok || len(prompts) == 0 {
		t.Fatalf("prompts/list prompts = %#v", promptsResult["prompts"])
	}
}

func TestMCP_ToolCall(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	responses, result := mcpConversation(t, sb, []string{"gabriel-greeting-go"}, []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "gabriel-greeting-go.GreetingService.SayHello",
				"arguments": map[string]any{
					"name":      "World",
					"lang_code": "en",
				},
			},
		},
	})
	requireSuccess(t, result)

	callResult, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call result = %#v", responses[1]["result"])
	}
	structured, ok := callResult["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent = %#v", callResult["structuredContent"])
	}
	if structured["greeting"] == "" {
		t.Fatalf("empty greeting in structuredContent: %#v", structured)
	}
}

func TestMCP_MultipleHolons(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	responses, result := mcpConversation(t, sb, []string{"gabriel-greeting-go", "gabriel-greeting-c"}, []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]any{},
		},
	})
	requireSuccess(t, result)

	toolsResult := responses[1]["result"].(map[string]any)
	tools := toolsResult["tools"].([]any)

	foundGo := false
	foundC := false
	for _, entry := range tools {
		name := entry.(map[string]any)["name"].(string)
		if name == "gabriel-greeting-go.GreetingService.SayHello" {
			foundGo = true
		}
		if name == "gabriel-greeting-c.GreetingService.SayHello" {
			foundC = true
		}
	}
	if !foundGo || !foundC {
		t.Fatalf("multi-holon tool list missing expected tools: %#v", tools)
	}
}

func TestMCP_NoArgsFails(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "mcp")
	requireFailure(t, result)
	requireContains(t, result.Stderr, "requires at least 1 arg")
}
