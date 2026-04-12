package invoke_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

// TestInvoke_CLI_MultiCall_JSONLines verifies that two calls produce two
// separate JSON Lines records on stdout, both valid JSON.
func TestInvoke_CLI_MultiCall_JSONLines(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "gabriel-greeting-go")

	result := sb.RunOP(t,
		"invoke", "stdio://gabriel-greeting-go",
		"SayHello", `{"name":"Alice","lang_code":"en"}`,
		"SayHello", `{"name":"Bob","lang_code":"fr"}`,
	)
	integration.RequireSuccess(t, result)

	lines := nonEmptyLines(result.Stdout)
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSON Lines, got %d:\n%s", len(lines), result.Stdout)
	}
	for i, line := range lines {
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("line %d is not valid JSON: %v\n%q", i+1, err, line)
		}
		if resp["greeting"] == "" {
			t.Fatalf("line %d: greeting is empty: %#v", i+1, resp)
		}
	}
}

// TestInvoke_CLI_MultiCall_SingleCallUnchanged verifies that N=1 output is
// valid JSON (unchanged format — may be pretty-printed or compact).
func TestInvoke_CLI_MultiCall_SingleCallUnchanged(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "gabriel-greeting-go")

	result := sb.RunOP(t,
		"invoke", "stdio://gabriel-greeting-go",
		"SayHello", `{"name":"World","lang_code":"en"}`,
	)
	integration.RequireSuccess(t, result)

	var resp map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &resp); err != nil {
		t.Fatalf("single-call output is not valid JSON: %v\n%s", err, result.Stdout)
	}
	if resp["greeting"] == "" {
		t.Fatalf("greeting is empty: %#v", resp)
	}
}

// TestInvoke_CLI_MultiCall_ImplicitEmptyPayload verifies that a method with
// no following JSON token is called with an implicit empty payload "{}".
func TestInvoke_CLI_MultiCall_ImplicitEmptyPayload(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "gabriel-greeting-go")

	// ListLanguages takes no arguments — omitting JSON must work.
	result := sb.RunOP(t,
		"invoke", "stdio://gabriel-greeting-go",
		"ListLanguages",
		"SayHello", `{"name":"World","lang_code":"en"}`,
	)
	integration.RequireSuccess(t, result)

	lines := nonEmptyLines(result.Stdout)
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSON Lines, got %d:\n%s", len(lines), result.Stdout)
	}
}

// TestInvoke_CLI_MultiCall_FailFast verifies that a failing first call stops
// execution: subsequent calls must not run, exit code must be non-zero.
func TestInvoke_CLI_MultiCall_FailFast(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "gabriel-greeting-go")

	result := sb.RunOP(t,
		"invoke", "stdio://gabriel-greeting-go",
		"DoesNotExist", `{}`,            // will fail
		"SayHello", `{"name":"World","lang_code":"en"}`, // must not run
	)
	integration.RequireFailure(t, result)

	// Ensure SayHello never ran (no greeting in any output line).
	for _, line := range nonEmptyLines(result.Stdout) {
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err == nil {
			if g, _ := resp["greeting"].(string); g != "" {
				t.Fatalf("SayHello ran after a failing call: %s", line)
			}
		}
	}
}

// TestInvoke_CLI_MultiCall_AcrossTransports runs a two-call demo across all
// available local transports (stdio, tcp, unix).
func TestInvoke_CLI_MultiCall_AcrossTransports(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "gabriel-greeting-go")

	for _, transport := range exampleInvokeTransports() {
		transport := transport
		t.Run(transport.Name, func(t *testing.T) {
			target, cleanup := startExampleTransportTarget(t, sb, "gabriel-greeting-go", transport)
			defer cleanup()

			result := sb.RunOP(t,
				"invoke", target,
				"SayHello", `{"name":"Alice","lang_code":"en"}`,
				"SayHello", `{"name":"Bob","lang_code":"fr"}`,
			)
			integration.RequireSuccess(t, result)

			lines := nonEmptyLines(result.Stdout)
			if len(lines) != 2 {
				t.Fatalf("[%s] expected 2 JSON Lines, got %d:\n%s",
					transport.Name, len(lines), result.Stdout)
			}
		})
	}
}
