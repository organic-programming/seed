package codex

import "testing"

func TestParseEvent(t *testing.T) {
	t.Parallel()

	timestamp, event, err := ParseEvent(`2026_03_10_19_15_03_042 {"type":"thread.started","thread_id":"thread-123"}`)
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	if timestamp != "2026_03_10_19_15_03_042" {
		t.Fatalf("unexpected timestamp: %q", timestamp)
	}
	if eventType(event) != "thread.started" {
		t.Fatalf("unexpected event type: %v", event["type"])
	}
	if event["thread_id"] != "thread-123" {
		t.Fatalf("unexpected thread id: %v", event["thread_id"])
	}
}

func TestExtractThreadIDAndTokenUsage(t *testing.T) {
	t.Parallel()

	events := []Event{
		{
			Timestamp: "2026_03_10_19_15_03_042",
			Data: map[string]any{
				"type":      "thread.started",
				"thread_id": "thread-123",
			},
		},
		{
			Timestamp: "2026_03_10_19_15_07_893",
			Data: map[string]any{
				"type": "turn.completed",
				"usage": map[string]any{
					"input_tokens":        10,
					"cached_input_tokens": 4,
					"output_tokens":       3,
				},
			},
		},
		{
			Timestamp: "2026_03_10_19_16_07_893",
			Data: map[string]any{
				"type": "turn.completed",
				"usage": map[string]any{
					"input_tokens":        5,
					"cached_input_tokens": 1,
					"output_tokens":       2,
				},
			},
		},
	}

	if threadID := ExtractThreadID(events); threadID != "thread-123" {
		t.Fatalf("unexpected thread id: %q", threadID)
	}

	usage := ExtractTokenUsage(events)
	if usage.InputTokens != 15 || usage.CachedInputTokens != 5 || usage.OutputTokens != 5 {
		t.Fatalf("unexpected token totals: %+v", usage)
	}
}
