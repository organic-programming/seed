package inspect

import (
	"fmt"
	"strings"
)

// RenderText formats a document for the human-readable op inspect output.
func RenderText(doc *Document) string {
	if doc == nil {
		return ""
	}

	var b strings.Builder
	switch {
	case doc.Slug != "" && doc.Motto != "":
		fmt.Fprintf(&b, "%s - %s\n", doc.Slug, doc.Motto)
	case doc.Slug != "":
		fmt.Fprintf(&b, "%s\n", doc.Slug)
	case doc.Motto != "":
		fmt.Fprintf(&b, "%s\n", doc.Motto)
	}

	if len(doc.Services) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		for i, service := range doc.Services {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "  %s\n", service.Name)
			if service.Description != "" {
				fmt.Fprintf(&b, "    %s\n", service.Description)
			}
			for _, method := range service.Methods {
				b.WriteString("\n")
				fmt.Fprintf(&b, "    %s(%s) -> %s\n", method.Name, shortTypeName(method.InputType), shortTypeName(method.OutputType))
				if method.Description != "" {
					fmt.Fprintf(&b, "      %s\n", method.Description)
				}
				if exampleInput := method.ExampleInput(); exampleInput != "" {
					fmt.Fprintf(&b, "      Example: %s\n", exampleInput)
				}
				if len(method.InputFields) > 0 {
					b.WriteString("\n")
					b.WriteString("      Request:\n")
					writeFieldBlock(&b, method.InputFields)
				}
				if len(method.OutputFields) > 0 {
					b.WriteString("\n")
					b.WriteString("      Response:\n")
					writeFieldBlock(&b, method.OutputFields)
				}
			}
		}
	}

	if len(doc.Skills) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("  Skills:\n")
		for _, skill := range doc.Skills {
			fmt.Fprintf(&b, "    %s", skill.Name)
			if skill.Description != "" {
				fmt.Fprintf(&b, " - %s", skill.Description)
			}
			b.WriteString("\n")
			if skill.When != "" {
				fmt.Fprintf(&b, "      When: %s\n", skill.When)
			}
			if len(skill.Steps) > 0 {
				b.WriteString("      Steps:\n")
				for i, step := range skill.Steps {
					fmt.Fprintf(&b, "        %d. %s\n", i+1, step)
				}
			}
		}
	}

	if len(doc.Sequences) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("  Sequences:\n")
		for _, sequence := range doc.Sequences {
			fmt.Fprintf(&b, "    %s", sequence.Name)
			if sequence.Description != "" {
				fmt.Fprintf(&b, " - %s", sequence.Description)
			}
			b.WriteString("\n")
			if len(sequence.Params) > 0 {
				b.WriteString("      Params:\n")
				for _, param := range sequence.Params {
					fmt.Fprintf(&b, "        %s", param.Name)
					if param.Required {
						b.WriteString(" [required]")
					}
					if param.Default != "" {
						fmt.Fprintf(&b, " default=%q", param.Default)
					}
					if param.Description != "" {
						fmt.Fprintf(&b, " - %s", param.Description)
					}
					b.WriteString("\n")
				}
			}
			if len(sequence.Steps) > 0 {
				b.WriteString("      Steps:\n")
				for i, step := range sequence.Steps {
					fmt.Fprintf(&b, "        %d. %s\n", i+1, step)
				}
			}
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeFieldBlock(b *strings.Builder, fields []Field) {
	nameWidth := 0
	typeWidth := 0
	reqWidth := len("[required]")

	for _, field := range fields {
		if len(field.Name) > nameWidth {
			nameWidth = len(field.Name)
		}
		fieldType := displayFieldType(field.Type)
		if len(fieldType) > typeWidth {
			typeWidth = len(fieldType)
		}
	}

	for _, field := range fields {
		fieldType := displayFieldType(field.Type)
		req := ""
		if field.Required {
			req = "[required]"
		}

		fmt.Fprintf(
			b,
			"        %-*s  %-*s  %-*s  %s\n",
			nameWidth,
			field.Name,
			typeWidth,
			fieldType,
			reqWidth,
			req,
			field.Description,
		)
		if field.Example != "" {
			prefix := strings.Repeat(" ", 8+nameWidth+2+typeWidth+2+reqWidth+2)
			fmt.Fprintf(b, "%s@example %s\n", prefix, field.Example)
		}
	}
}

func shortTypeName(name string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(name), ".")
	if trimmed == "" {
		return ""
	}
	if idx := strings.LastIndex(trimmed, "."); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

func displayFieldType(name string) string {
	trimmed := strings.TrimSpace(name)
	if strings.HasPrefix(trimmed, "map<") && strings.HasSuffix(trimmed, ">") {
		body := strings.TrimSuffix(strings.TrimPrefix(trimmed, "map<"), ">")
		parts := strings.SplitN(body, ",", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("map<%s, %s>", shortTypeName(parts[0]), shortTypeName(parts[1]))
		}
	}
	return shortTypeName(trimmed)
}
