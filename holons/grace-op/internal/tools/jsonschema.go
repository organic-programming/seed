package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
)

// Definition is a normalized tool definition shared by op tools and op mcp.
type Definition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// DefinitionsForCatalogs builds tool definitions for one or more holon
// catalogs. Names are namespaced by slug to avoid collisions.
func DefinitionsForCatalogs(catalogs []*inspectpkg.LocalCatalog) []Definition {
	out := make([]Definition, 0)
	for _, catalog := range catalogs {
		if catalog == nil {
			continue
		}
		for _, binding := range catalog.Methods {
			out = append(out, Definition{
				Name:        binding.ToolName(catalog.Slug),
				Description: strings.TrimSpace(binding.Method.Description),
				InputSchema: JSONSchemaForMethod(binding.Method),
			})
		}
		for _, sequence := range catalog.Document.Sequences {
			out = append(out, Definition{
				Name:        catalog.Slug + ".sequence." + sequence.Name,
				Description: strings.TrimSpace(sequence.Description),
				InputSchema: JSONSchemaForSequence(sequence),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func JSONSchemaForSequence(sequence inspectpkg.Sequence) map[string]any {
	properties := make(map[string]any, len(sequence.Params))
	required := make([]string, 0)

	for _, param := range sequence.Params {
		properties[param.Name] = map[string]any{
			"type": "string",
		}
		if strings.TrimSpace(param.Description) != "" {
			properties[param.Name].(map[string]any)["description"] = strings.TrimSpace(param.Description)
		}
		if strings.TrimSpace(param.Default) != "" {
			properties[param.Name].(map[string]any)["default"] = strings.TrimSpace(param.Default)
		}
		if param.Required {
			required = append(required, param.Name)
		}
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if strings.TrimSpace(sequence.Description) != "" {
		schema["description"] = strings.TrimSpace(sequence.Description)
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// JSONSchemaForMethod converts a parsed proto method input into JSON Schema.
func JSONSchemaForMethod(method inspectpkg.Method) map[string]any {
	properties := make(map[string]any, len(method.InputFields))
	required := make([]string, 0)

	for _, field := range method.InputFields {
		properties[field.Name] = schemaForField(field)
		if field.Required {
			required = append(required, field.Name)
		}
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if strings.TrimSpace(method.Description) != "" {
		schema["description"] = strings.TrimSpace(method.Description)
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	if example, ok := parseExample(method.ExampleInput()); ok {
		schema["examples"] = []any{example}
	}
	return schema
}

func schemaForField(field inspectpkg.Field) map[string]any {
	var schema map[string]any
	switch field.Label {
	case inspectpkg.FieldLabelMap:
		schema = map[string]any{
			"type":                 "object",
			"additionalProperties": schemaForType(field.MapValueType, field.NestedFields, field.EnumValues),
		}
	case inspectpkg.FieldLabelRepeated:
		schema = map[string]any{
			"type":  "array",
			"items": schemaForType(field.Type, field.NestedFields, field.EnumValues),
		}
	default:
		schema = schemaForType(field.Type, field.NestedFields, field.EnumValues)
	}

	if strings.TrimSpace(field.Description) != "" {
		schema["description"] = strings.TrimSpace(field.Description)
	}
	if example, ok := parseExample(field.Example); ok {
		schema["examples"] = []any{example}
	}
	return schema
}

func schemaForType(typeName string, nested []inspectpkg.Field, enumValues []inspectpkg.EnumValue) map[string]any {
	if len(enumValues) > 0 {
		values := make([]any, 0, len(enumValues))
		for _, value := range enumValues {
			values = append(values, value.Name)
		}
		return map[string]any{
			"type": "string",
			"enum": values,
		}
	}

	if len(nested) > 0 {
		properties := make(map[string]any, len(nested))
		required := make([]string, 0)
		for _, field := range nested {
			properties[field.Name] = schemaForField(field)
			if field.Required {
				required = append(required, field.Name)
			}
		}
		schema := map[string]any{
			"type":                 "object",
			"properties":           properties,
			"additionalProperties": false,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}

	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "string":
		return map[string]any{"type": "string"}
	case "int32", "int64", "uint32", "uint64", "sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64":
		return map[string]any{"type": "integer"}
	case "float", "double":
		return map[string]any{"type": "number"}
	case "bool":
		return map[string]any{"type": "boolean"}
	case "bytes":
		return map[string]any{"type": "string", "format": "byte"}
	default:
		if strings.HasPrefix(typeName, "map<") {
			return map[string]any{"type": "object"}
		}
		if strings.Contains(typeName, ".") {
			return map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			}
		}
		return map[string]any{
			"type":         "string",
			"x-proto-type": fmt.Sprintf("%s", strings.TrimSpace(typeName)),
		}
	}
}

func parseExample(raw string) (any, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false
	}

	var value any
	if err := json.Unmarshal([]byte(trimmed), &value); err == nil {
		return value, true
	}

	return trimmed, true
}
