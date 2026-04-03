package runner

import (
	"context"
	"reflect"
	"testing"

	"github.com/organic-programming/james-loops/internal/profile"
)

func TestCodexRunner_BuildsCorrectCommand(t *testing.T) {
	restore, gotRepoRoot, gotSpec := captureCommand(t)
	defer restore()

	item := profile.Profile{
		Name:      "codex-default",
		Driver:    profile.DriverCodex,
		Model:     "o4-mini",
		ExtraArgs: []string{"--full-auto", "-a", "never"},
	}
	r, err := New(item)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, _, _, err := r.Run(context.Background(), "/repo", "hello"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if *gotRepoRoot != "/repo" {
		t.Fatalf("repo root = %q, want /repo", *gotRepoRoot)
	}
	want := commandSpec{
		Name: "codex",
		Args: []string{"exec", "--full-auto", "-a", "never", "--model", "o4-mini", "-C", "/repo", "hello"},
	}
	if !reflect.DeepEqual(*gotSpec, want) {
		t.Fatalf("command = %#v, want %#v", *gotSpec, want)
	}
}

func TestCodexRunner_IsQuotaIssue(t *testing.T) {
	r := codexRunner{profile: profile.Profile{Driver: profile.DriverCodex}}
	if !r.IsQuotaIssue(1, nil, []byte("rate limit exceeded")) {
		t.Fatal("expected quota issue to be detected")
	}
}

func TestGeminiRunner_BuildsCorrectCommand(t *testing.T) {
	t.Run("without model", func(t *testing.T) {
		restore, _, gotSpec := captureCommand(t)
		defer restore()
		r := geminiRunner{profile: profile.Profile{Driver: profile.DriverGemini}}
		if _, _, _, err := r.Run(context.Background(), "/repo", "hello"); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := commandSpec{Name: "gemini", Args: []string{"-p", "hello"}}
		if !reflect.DeepEqual(*gotSpec, want) {
			t.Fatalf("command = %#v, want %#v", *gotSpec, want)
		}
	})

	t.Run("with model", func(t *testing.T) {
		restore, _, gotSpec := captureCommand(t)
		defer restore()
		r := geminiRunner{profile: profile.Profile{Driver: profile.DriverGemini, Model: "gemini-2.5-pro"}}
		if _, _, _, err := r.Run(context.Background(), "/repo", "hello"); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := commandSpec{Name: "gemini", Args: []string{"-p", "hello", "-m", "gemini-2.5-pro"}}
		if !reflect.DeepEqual(*gotSpec, want) {
			t.Fatalf("command = %#v, want %#v", *gotSpec, want)
		}
	})
}

func TestGeminiRunner_AddsOutputFormat(t *testing.T) {
	item := WithGeminiJSONOutput(profile.Profile{Driver: profile.DriverGemini})
	want := []string{"--output-format", "json"}
	if !reflect.DeepEqual(item.ExtraArgs, want) {
		t.Fatalf("extra args = %v, want %v", item.ExtraArgs, want)
	}
}

func TestOllamaRunner_NeverReportsQuota(t *testing.T) {
	r := ollamaRunner{profile: profile.Profile{Driver: profile.DriverOllama, Model: "llama3.3"}}
	if r.IsQuotaIssue(1, nil, []byte("quota")) {
		t.Fatal("ollama runner should never report quota")
	}
}

func captureCommand(t *testing.T) (restore func(), gotRepoRoot *string, gotSpec *commandSpec) {
	t.Helper()
	var repoRoot string
	var spec commandSpec
	prev := execRunnerCommand
	execRunnerCommand = func(ctx context.Context, root string, cmd commandSpec) (int, []byte, []byte, error) {
		repoRoot = root
		spec = cmd
		return 0, []byte("ok"), nil, nil
	}
	return func() {
		execRunnerCommand = prev
	}, &repoRoot, &spec
}
