package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/organic-programming/codex-orchestrator/internal/state"
)

type Event struct {
	Timestamp string
	Data      map[string]any
}

func ParseEvent(line string) (string, map[string]any, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil, fmt.Errorf("parse event: empty line")
	}

	jsonStart := strings.IndexByte(line, '{')
	if jsonStart == -1 {
		return "", nil, fmt.Errorf("parse event: missing JSON object")
	}

	timestamp := strings.TrimSpace(line[:jsonStart])
	if timestamp == "" {
		return "", nil, fmt.Errorf("parse event: missing timestamp")
	}

	payload := make(map[string]any)
	decoder := json.NewDecoder(strings.NewReader(line[jsonStart:]))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", nil, fmt.Errorf("parse event JSON: %w", err)
	}

	return timestamp, payload, nil
}

func ReadEvents(logFile string) ([]Event, error) {
	file, err := os.Open(logFile)
	if err != nil {
		return nil, fmt.Errorf("open JSONL log: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var events []Event
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		timestamp, payload, err := ParseEvent(line)
		if err != nil {
			return nil, err
		}

		events = append(events, Event{
			Timestamp: timestamp,
			Data:      payload,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan JSONL log: %w", err)
	}

	return events, nil
}

func ExtractThreadID(events []Event) string {
	for _, event := range events {
		if eventType(event.Data) != "thread.started" {
			continue
		}

		if threadID, ok := event.Data["thread_id"].(string); ok {
			return threadID
		}
	}

	return ""
}

func ExtractTokenUsage(events []Event) state.TokenUsage {
	var total state.TokenUsage

	for _, event := range events {
		if eventType(event.Data) != "turn.completed" {
			continue
		}

		usage, ok := event.Data["usage"].(map[string]any)
		if !ok {
			continue
		}

		total.InputTokens += asInt(usage["input_tokens"])
		total.CachedInputTokens += asInt(usage["cached_input_tokens"])
		total.OutputTokens += asInt(usage["output_tokens"])
	}

	return total
}

func eventType(payload map[string]any) string {
	value, _ := payload["type"].(string)
	return value
}

func asInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n)
		}
	}

	return 0
}
