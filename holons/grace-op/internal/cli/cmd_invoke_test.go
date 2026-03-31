package cli

import (
	"reflect"
	"strings"
	"testing"

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
