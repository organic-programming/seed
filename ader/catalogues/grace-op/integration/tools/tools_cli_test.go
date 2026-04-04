package tools_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestTools_CLI_Formats(t *testing.T) {
	formats := []string{"openai", "anthropic", "mcp"}

	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			for _, format := range formats {
				t.Run(format, func(t *testing.T) {
					result := sb.RunOP(t, "tools", spec.Slug, "--format", format)
					integration.RequireSuccess(t, result)

					payload := integration.DecodeJSON[[]map[string]any](t, result.Stdout)
					if len(payload) == 0 {
						t.Fatalf("tools %s returned no definitions", format)
					}
				})
			}
		})
	}
}

func TestTools_CLI_DefaultFormat(t *testing.T) {
	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			result := sb.RunOP(t, "tools", spec.Slug)
			integration.RequireSuccess(t, result)

			payload := integration.DecodeJSON[[]map[string]any](t, result.Stdout)
			if len(payload) == 0 {
				t.Fatalf("default tools payload empty for %s", spec.Slug)
			}
			if _, ok := payload[0]["function"]; !ok {
				t.Fatalf("default tools payload is not OpenAI-shaped: %#v", payload[0])
			}
		})
	}
}
