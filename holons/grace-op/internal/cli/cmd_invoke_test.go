package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	internalgrpc "github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/spf13/cobra"
)

func TestNextExampleToken(t *testing.T) {
	seq := []string{`{"name":"Bob","lang_code":"fr"}`, "--origin"}

	tests := []struct {
		name       string
		typed      []string
		toComplete string
		want       string
		wantOK     bool
	}{
		{
			name:   "match position 0",
			typed:  nil,
			want:   "'{\"name\":\"Bob\",\"lang_code\":\"fr\"}'",
			wantOK: true,
		},
		{
			name:   "match position 1",
			typed:  []string{`'{"name":"Bob","lang_code":"fr"}'`},
			want:   "--origin",
			wantOK: true,
		},
		{
			name:   "exhausted",
			typed:  []string{`'{"name":"Bob","lang_code":"fr"}'`, "--origin"},
			wantOK: false,
		},
		{
			name:   "no match",
			typed:  []string{`'{"name":"Alice","lang_code":"fr"}'`},
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := nextExampleToken(seq, tc.typed, tc.toComplete)
			if ok != tc.wantOK {
				t.Fatalf("nextExampleToken ok = %t, want %t", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("nextExampleToken = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompleteInvokePayload(t *testing.T) {
	repoRoot := inspectRepoRoot(t)
	chdirForTest(t, repoRoot)

	root := newRootCmd("0.1.0-test")
	invokeCmd := mustFindSubcommand(t, root, "invoke")

	initial := completeInvokePayload(invokeCmd, "gabriel-greeting-go", "SayHello", nil, "")
	assertCompletionContains(t, initial, "'{\"name\":\"Bob\",\"lang_code\":\"fr\"}'")

	afterJSON := completeInvokePayload(
		invokeCmd,
		"gabriel-greeting-go",
		"SayHello",
		[]string{`'{"name":"Bob","lang_code":"fr"}'`},
		"",
	)
	assertCompletionContains(t, afterJSON, "--origin")
	assertCompletionNotContains(t, afterJSON, "'{\"name\":\"Bob\",\"lang_code\":\"fr\"}'")

	exhausted := completeInvokePayload(
		invokeCmd,
		"gabriel-greeting-go",
		"SayHello",
		[]string{`'{"name":"Bob","lang_code":"fr"}'`, "--origin"},
		"",
	)
	assertCompletionNotContains(t, exhausted, "--origin")

	listLanguages := completeInvokePayload(invokeCmd, "gabriel-greeting-go", "ListLanguages", nil, "")
	assertCompletionNotContains(t, listLanguages, "{}")
	assertCompletionNotContains(t, listLanguages, "'{}'")
}

func TestInvokeAndRootFallbackPayloadCompletionShareExamples(t *testing.T) {
	repoRoot := inspectRepoRoot(t)
	chdirForTest(t, repoRoot)

	root := newRootCmd("0.1.0-test")
	invokeCmd := mustFindSubcommand(t, root, "invoke")

	invokeInitial, _ := completeInvokeArgs(invokeCmd, []string{"gabriel-greeting-go", "SayHello"}, "")
	rootInitial, _ := completeRootFallbackArgs(root, []string{"gabriel-greeting-go", "SayHello"}, "")
	assertCompletionContains(t, invokeInitial, "'{\"name\":\"Bob\",\"lang_code\":\"fr\"}'")
	assertCompletionContains(t, rootInitial, "'{\"name\":\"Bob\",\"lang_code\":\"fr\"}'")
	if !reflect.DeepEqual(invokeInitial, rootInitial) {
		t.Fatalf("initial completions differ:\ninvoke=%#v\nroot=%#v", invokeInitial, rootInitial)
	}

	invokeAfterJSON, _ := completeInvokeArgs(
		invokeCmd,
		[]string{"gabriel-greeting-go", "SayHello", "'{\"name\":\"Bob\",\"lang_code\":\"fr\"}'"},
		"",
	)
	rootAfterJSON, _ := completeRootFallbackArgs(
		root,
		[]string{"gabriel-greeting-go", "SayHello", "'{\"name\":\"Bob\",\"lang_code\":\"fr\"}'"},
		"",
	)
	assertCompletionContains(t, invokeAfterJSON, "--origin")
	assertCompletionContains(t, rootAfterJSON, "--origin")
	if !reflect.DeepEqual(invokeAfterJSON, rootAfterJSON) {
		t.Fatalf("post-json completions differ:\ninvoke=%#v\nroot=%#v", invokeAfterJSON, rootAfterJSON)
	}
}

func mustFindSubcommand(t *testing.T, cmd *cobra.Command, name string) *cobra.Command {
	t.Helper()

	for _, child := range cmd.Commands() {
		if child.Name() == name {
			return child
		}
	}
	t.Fatalf("subcommand %q not found", name)
	return nil
}

func assertCompletionContains(t *testing.T, got []string, want string) {
	t.Helper()
	for _, candidate := range got {
		if completionValue(candidate) == want {
			return
		}
	}
	t.Fatalf("completion missing %q: %#v", want, got)
}

func assertCompletionNotContains(t *testing.T, got []string, want string) {
	t.Helper()
	for _, candidate := range got {
		if completionValue(candidate) == want {
			t.Fatalf("completion unexpectedly contains %q: %#v", want, got)
		}
	}
}

func completionValue(candidate string) string {
	if idx := strings.IndexRune(candidate, '\t'); idx >= 0 {
		return candidate[:idx]
	}
	return candidate
}

func TestParseInvokeCalls(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []invokeCall
		wantErr string
	}{
		{
			name: "single method no payload",
			args: []string{"Ping"},
			want: []invokeCall{{method: "Ping", inputJSON: "{}"}},
		},
		{
			name: "single method with payload",
			args: []string{"Ping", `{"ok":true}`},
			want: []invokeCall{{method: "Ping", inputJSON: `{"ok":true}`}},
		},
		{
			name: "two methods with payloads",
			args: []string{"Add", `{"v":1}`, "Add", `{"v":10}`},
			want: []invokeCall{
				{method: "Add", inputJSON: `{"v":1}`},
				{method: "Add", inputJSON: `{"v":10}`},
			},
		},
		{
			name: "two methods first has no payload",
			args: []string{"Ping", "Add", `{"v":1}`},
			want: []invokeCall{
				{method: "Ping", inputJSON: "{}"},
				{method: "Add", inputJSON: `{"v":1}`},
			},
		},
		{
			name: "five-op calculator sequence",
			args: []string{
				"Set", `{"value":20.0}`,
				"Add", `{"value":1.0}`,
				"Subtract", `{"value":4.0}`,
				"Divide", `{"by":5.0}`,
				"Multiply", `{"by":3.0}`,
			},
			want: []invokeCall{
				{method: "Set", inputJSON: `{"value":20.0}`},
				{method: "Add", inputJSON: `{"value":1.0}`},
				{method: "Subtract", inputJSON: `{"value":4.0}`},
				{method: "Divide", inputJSON: `{"by":5.0}`},
				{method: "Multiply", inputJSON: `{"by":3.0}`},
			},
		},
		{
			name:    "empty args",
			args:    []string{},
			wantErr: "method required",
		},
		{
			name:    "first arg is JSON",
			args:    []string{`{"v":1}`},
			wantErr: "first token must be a method name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseInvokeCalls(tc.args)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("parseInvokeCalls(%v) = nil error, want error containing %q", tc.args, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseInvokeCalls(%v) error = %q, want containing %q", tc.args, err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInvokeCalls(%v) unexpected error: %v", tc.args, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseInvokeCalls(%v)\ngot:  %+v\nwant: %+v", tc.args, got, tc.want)
			}
		})
	}
}

func TestMultiCallOutputIsJSONLines(t *testing.T) {
	calls := []invokeCall{
		{method: "Ping", inputJSON: `{"value":1}`},
		{method: "Ping", inputJSON: `{"value":2}`},
	}

	output := captureStdout(t, func() {
		code := emitInvokeResults(FormatJSON, "op invoke", calls, func(index int, call invokeCall) (*internalgrpc.CallResult, error) {
			return &internalgrpc.CallResult{
				Output: "{\n  \"method\": \"" + call.method + "\",\n  \"value\": " + string('1'+rune(index)) + "\n}\n",
			}, nil
		})
		if code != 0 {
			t.Fatalf("emitInvokeResults returned %d, want 0", code)
		}
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSON Lines, got %d:\n%s", len(lines), output)
	}
	for i, line := range lines {
		if strings.Contains(line, "\t") || strings.Contains(line, "  ") {
			t.Fatalf("line %d is not compact JSON: %q", i+1, line)
		}
		if !json.Valid([]byte(line)) {
			t.Fatalf("line %d is not valid JSON: %q", i+1, line)
		}
	}
}
