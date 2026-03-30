package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const protoSchemaV1 = "holon/v1"

func normalizeProtoSchema(schema string) string {
	trimmed := strings.TrimSpace(schema)
	if trimmed == "" || trimmed == "holon/v0" {
		return protoSchemaV1
	}
	return trimmed
}

func appendProtoStringField(b *strings.Builder, indent, name, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	fmt.Fprintf(b, "%s%s: %s\n", indent, name, strconv.Quote(trimmed))
}

func appendProtoAliasesField(b *strings.Builder, indent string, aliases []string) {
	aliases = compactStrings(aliases)
	if len(aliases) == 0 {
		return
	}

	b.WriteString(indent)
	b.WriteString("aliases: [")
	for i, alias := range aliases {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(alias))
	}
	b.WriteString("]\n")
}

// WriteHolonProto renders an Identity to a holon.proto file at the given path.
func WriteHolonProto(id Identity, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("proto path is required")
	}

	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\n\n")
	b.WriteString("import \"holons/v1/manifest.proto\";\n\n")
	b.WriteString("option (holons.v1.manifest) = {\n")
	b.WriteString("  identity: {\n")
	appendProtoStringField(&b, "    ", "schema", normalizeProtoSchema(id.Schema))
	appendProtoStringField(&b, "    ", "uuid", id.UUID)
	appendProtoStringField(&b, "    ", "given_name", id.GivenName)
	appendProtoStringField(&b, "    ", "family_name", id.FamilyName)
	appendProtoStringField(&b, "    ", "motto", id.Motto)
	appendProtoStringField(&b, "    ", "composer", id.Composer)
	appendProtoStringField(&b, "    ", "status", id.Status)
	appendProtoStringField(&b, "    ", "born", id.Born)
	appendProtoAliasesField(&b, "    ", id.Aliases)
	b.WriteString("  }\n")
	appendProtoStringField(&b, "  ", "description", id.Description)
	appendProtoStringField(&b, "  ", "lang", id.Lang)
	b.WriteString("};\n")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}
