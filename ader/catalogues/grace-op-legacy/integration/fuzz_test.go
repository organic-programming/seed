// Fuzz targets send random commands and payloads to the real binary and assert
// the process does not hang or crash.
package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

const fuzzOptInReason = "set OP_TEST_FUZZ=1 to run fuzz targets"

func requireFuzzOptIn(f *testing.F) {
	f.Helper()
	if testing.Short() {
		f.Skip(shortTestReason)
	}
	if strings.TrimSpace(os.Getenv("OP_TEST_FUZZ")) != "1" {
		f.Skip(fuzzOptInReason)
	}
}

func FuzzRandomCommands(f *testing.F) {
	requireFuzzOptIn(f)

	for _, seed := range []string{
		"version",
		"discover",
		"build --dry-run gabriel-greeting-go",
		"tools gabriel-greeting-go --format openai",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, args string) {
		sb := newSandbox(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result := sb.runOPWithOptions(t, runOptions{Context: ctx}, strings.Fields(args)...)
		if result.TimedOut {
			t.Fatalf("command timed out: %q", args)
		}
	})
}

func FuzzJSONInput(f *testing.F) {
	requireFuzzOptIn(f)

	for _, seed := range []string{`{"name":"World","lang_code":"en"}`, "{broken", "[]"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, payload string) {
		sb := newSandbox(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result := sb.runOPWithOptions(t, runOptions{Context: ctx}, "gabriel-greeting-go", "SayHello", payload)
		if result.TimedOut {
			t.Fatalf("json fuzz timed out: %q", payload)
		}
	})
}

func FuzzTransportURI(f *testing.F) {
	requireFuzzOptIn(f)

	for _, seed := range []string{"grpc://gabriel-greeting-go", "tcp://gabriel-greeting-go", "stdio://gabriel-greeting-go", "bogus://gabriel-greeting-go"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, target string) {
		sb := newSandbox(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result := sb.runOPWithOptions(t, runOptions{Context: ctx}, target, "SayHello", `{"name":"World","lang_code":"en"}`)
		if result.TimedOut {
			t.Fatalf("transport fuzz timed out: %q", target)
		}
	})
}

func FuzzFlagPermutations(f *testing.F) {
	requireFuzzOptIn(f)

	for _, seed := range []string{
		"invoke gabriel-greeting-go SayHello --no-build {}",
		"run --no-build gabriel-greeting-go",
		"build --dry-run gabriel-greeting-go",
		"build --clean --dry-run gabriel-greeting-go",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, args string) {
		sb := newSandbox(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result := sb.runOPWithOptions(t, runOptions{Context: ctx}, strings.Fields(args)...)
		if result.TimedOut {
			t.Fatalf("flag fuzz timed out: %q", args)
		}
	})
}
