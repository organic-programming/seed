package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	FormatOpenAI    = "openai"
	FormatAnthropic = "anthropic"
	FormatMCP       = "mcp"
)

// ParseFormat validates the op tools output format.
func ParseFormat(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", FormatOpenAI:
		return FormatOpenAI, nil
	case FormatAnthropic:
		return FormatAnthropic, nil
	case FormatMCP:
		return FormatMCP, nil
	default:
		return "", fmt.Errorf("invalid --format %q (supported: openai, anthropic, mcp)", value)
	}
}

// MarshalDefinitions renders normalized tool definitions into the requested
// agent-specific output shape.
func MarshalDefinitions(definitions []Definition, format string) ([]byte, error) {
	parsed, err := ParseFormat(format)
	if err != nil {
		return nil, err
	}

	payload := make([]any, 0, len(definitions))
	for _, definition := range definitions {
		switch parsed {
		case FormatOpenAI:
			payload = append(payload, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        definition.Name,
					"description": definition.Description,
					"parameters":  definition.InputSchema,
				},
			})
		case FormatAnthropic:
			payload = append(payload, map[string]any{
				"name":         definition.Name,
				"description":  definition.Description,
				"input_schema": definition.InputSchema,
			})
		case FormatMCP:
			payload = append(payload, map[string]any{
				"name":        definition.Name,
				"description": definition.Description,
				"inputSchema": definition.InputSchema,
			})
		}
	}

	return json.MarshalIndent(payload, "", "  ")
}
