package integration

import "testing"

func TestTools_Formats(t *testing.T) {
	formats := []string{"openai", "anthropic", "mcp"}

	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)
			for _, format := range formats {
				t.Run(format, func(t *testing.T) {
					result := sb.runOP(t, "tools", spec.Slug, "--format", format)
					requireSuccess(t, result)

					var payload []map[string]any
					payload = decodeJSON[[]map[string]any](t, result.Stdout)
					if len(payload) == 0 {
						t.Fatalf("tools %s returned no definitions", format)
					}
				})
			}
		})
	}
}

func TestTools_DefaultFormat(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)
			result := sb.runOP(t, "tools", spec.Slug)
			requireSuccess(t, result)

			var payload []map[string]any
			payload = decodeJSON[[]map[string]any](t, result.Stdout)
			if len(payload) == 0 {
				t.Fatalf("default tools payload empty for %s", spec.Slug)
			}
			if _, ok := payload[0]["function"]; !ok {
				t.Fatalf("default tools payload is not OpenAI-shaped: %#v", payload[0])
			}
		})
	}
}
