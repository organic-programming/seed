//go:build e2e

package invoke_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

// TestInvoke_CLI_Calculator_DemoSequence builds matt-calculator-go via op,
// then runs the full 5-operation demo sequence and verifies each result value.
func TestInvoke_CLI_Calculator_DemoSequence(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "matt-calculator-go")

	result := sb.RunOP(t,
		"invoke", "stdio://matt-calculator-go",
		"Set", `{"value":20.0}`,
		"Add", `{"value":1.0}`,
		"Subtract", `{"value":4.0}`,
		"Divide", `{"by":5.0}`,
		"Multiply", `{"by":3.0}`,
	)
	integration.RequireSuccess(t, result)

	lines := nonEmptyLines(result.Stdout)
	if len(lines) != 5 {
		t.Fatalf("expected 5 JSON Lines, got %d:\n%s", len(lines), result.Stdout)
	}

	type calcResp struct {
		Result     float64 `json:"result"`
		Expression string  `json:"expression"`
	}
	expected := []float64{20, 21, 17, 3.4, 10.2}

	for i, line := range lines {
		var resp calcResp
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("line %d not valid JSON: %v\n%q", i+1, err, line)
		}
		if math.Abs(resp.Result-expected[i]) > 1e-9 {
			t.Fatalf("line %d: result = %v, want %v", i+1, resp.Result, expected[i])
		}
		if resp.Expression == "" {
			t.Fatalf("line %d: expression is empty", i+1)
		}
	}
}

// TestInvoke_CLI_Calculator_DivideByZero verifies that Divide with by=0
// produces a non-zero exit code and writes an error mentioning "zero" to stderr.
func TestInvoke_CLI_Calculator_DivideByZero(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "matt-calculator-go")

	result := sb.RunOP(t,
		"invoke", "stdio://matt-calculator-go",
		"Divide", `{"by":0}`,
	)
	integration.RequireFailure(t, result)
	integration.RequireContains(t, result.Stderr, "zero")
}

// TestInvoke_CLI_Calculator_AccumulatorResetsBetweenInvocations verifies that
// two separate op invoke calls each start with the accumulator at 0.0.
func TestInvoke_CLI_Calculator_AccumulatorResetsBetweenInvocations(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.BuildReportFor(t, sb, "matt-calculator-go")

	addTen := func() float64 {
		t.Helper()
		result := sb.RunOP(t,
			"invoke", "stdio://matt-calculator-go",
			"Add", `{"value":10.0}`,
		)
		integration.RequireSuccess(t, result)
		var resp struct {
			Result float64 `json:"result"`
		}
		resp = integration.DecodeJSON[struct {
			Result float64 `json:"result"`
		}](t, result.Stdout)
		return resp.Result
	}

	first := addTen()
	second := addTen()
	if first != 10.0 || second != 10.0 {
		t.Fatalf("accumulator was not reset between invocations: first=%v second=%v", first, second)
	}
}
