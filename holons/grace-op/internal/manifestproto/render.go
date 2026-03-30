package manifestproto

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	templateNamePrefix = "holon."
	templateLegacyExt  = "yaml.tmpl"
	templateProtoName  = "holon.proto"
)

type RenderOptions struct {
	FallbackName string
}

type manifest struct {
	Schema      string     `yaml:"schema"`
	UUID        string     `yaml:"uuid"`
	GivenName   string     `yaml:"given_name"`
	FamilyName  string     `yaml:"family_name"`
	Motto       string     `yaml:"motto"`
	Composer    string     `yaml:"composer"`
	Status      string     `yaml:"status"`
	Born        string     `yaml:"born"`
	Aliases     []string   `yaml:"aliases"`
	Description string     `yaml:"description"`
	Lang        string     `yaml:"lang"`
	Kind        string     `yaml:"kind"`
	Platforms   []string   `yaml:"platforms"`
	Transport   string     `yaml:"transport"`
	Skills      []skill    `yaml:"skills"`
	Sequences   []sequence `yaml:"sequences"`
	Contract    *contract  `yaml:"contract"`
	Build       build      `yaml:"build"`
	Requires    requires   `yaml:"requires"`
	Delegates   delegates  `yaml:"delegates"`
	Artifacts   artifacts  `yaml:"artifacts"`
}

type skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	When        string   `yaml:"when"`
	Steps       []string `yaml:"steps"`
}

type sequence struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Params      []sequenceParam `yaml:"params"`
	Steps       []string        `yaml:"steps"`
}

type sequenceParam struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"`
}

type contract struct {
	Proto   string   `yaml:"proto"`
	Service string   `yaml:"service"`
	RPCs    []string `yaml:"rpcs"`
	GRPC    bool     `yaml:"grpc"`
}

type build struct {
	Runner   string                 `yaml:"runner"`
	Main     string                 `yaml:"main"`
	Defaults *defaults              `yaml:"defaults"`
	Members  []member               `yaml:"members"`
	Targets  map[string]buildTarget `yaml:"targets"`
}

type defaults struct {
	Target string `yaml:"target"`
	Mode   string `yaml:"mode"`
}

type member struct {
	ID   string `yaml:"id"`
	Path string `yaml:"path"`
	Type string `yaml:"type"`
}

type buildTarget struct {
	Steps []step `yaml:"steps"`
}

type step struct {
	BuildMember  string        `yaml:"build_member"`
	Exec         *execStep     `yaml:"exec"`
	Copy         *copyStep     `yaml:"copy"`
	AssertFile   *fileStep     `yaml:"assert_file"`
	CopyArtifact *artifactStep `yaml:"copy_artifact"`
}

type execStep struct {
	Cwd  string   `yaml:"cwd"`
	Argv []string `yaml:"argv"`
}

type copyStep struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type fileStep struct {
	Path string `yaml:"path"`
}

type artifactStep struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type requires struct {
	Commands []string `yaml:"commands"`
	Files    []string `yaml:"files"`
}

type delegates struct {
	Commands []string `yaml:"commands"`
}

type artifacts struct {
	Binary  string `yaml:"binary"`
	Primary string `yaml:"primary"`
}

func IsLegacyTemplatePath(path string) bool {
	return filepath.Base(path) == templateNamePrefix+templateLegacyExt
}

func OutputFileName() string {
	return templateProtoName
}

func RenderFromYAML(data []byte, opts RenderOptions) ([]byte, error) {
	var legacy manifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&legacy); err != nil {
		return nil, fmt.Errorf("parse manifest template: %w", err)
	}

	givenName, familyName := strings.TrimSpace(legacy.GivenName), strings.TrimSpace(legacy.FamilyName)
	if givenName == "" && familyName == "" {
		givenName, familyName = fallbackIdentity(opts.FallbackName)
	}

	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\n\n")
	b.WriteString("import \"holons/v1/manifest.proto\";\n\n")
	b.WriteString("option (holons.v1.manifest) = {\n")
	b.WriteString("  identity: {\n")
	appendStringField(&b, "    ", "schema", normalizeSchema(legacy.Schema))
	appendStringField(&b, "    ", "uuid", legacy.UUID)
	appendStringField(&b, "    ", "given_name", givenName)
	appendStringField(&b, "    ", "family_name", familyName)
	appendStringField(&b, "    ", "motto", legacy.Motto)
	appendStringField(&b, "    ", "composer", legacy.Composer)
	appendStringField(&b, "    ", "status", legacy.Status)
	appendStringField(&b, "    ", "born", legacy.Born)
	appendStringSliceField(&b, "    ", "aliases", legacy.Aliases)
	b.WriteString("  }\n")

	appendStringField(&b, "  ", "description", legacy.Description)
	appendStringField(&b, "  ", "lang", legacy.Lang)
	appendRepeatedSkills(&b, legacy.Skills)
	appendContract(&b, legacy.Contract)
	appendStringField(&b, "  ", "kind", legacy.Kind)
	appendStringSliceField(&b, "  ", "platforms", legacy.Platforms)
	appendStringField(&b, "  ", "transport", legacy.Transport)
	appendBuild(&b, legacy.Build)
	appendRequires(&b, legacy.Requires, legacy.Delegates)
	appendArtifacts(&b, legacy.Artifacts)
	appendRepeatedSequences(&b, legacy.Sequences)
	b.WriteString("};\n")

	return []byte(b.String()), nil
}

func normalizeSchema(schema string) string {
	trimmed := strings.TrimSpace(schema)
	if trimmed == "" || trimmed == "holon/v0" {
		return "holon/v1"
	}
	return trimmed
}

func fallbackIdentity(name string) (string, string) {
	trimmed := strings.TrimSpace(filepath.Base(name))
	trimmed = strings.Trim(trimmed, "-_. ")
	if trimmed == "" {
		return "Proto", "Holon"
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(parts) == 0 {
		return "Proto", "Holon"
	}
	if len(parts) == 1 {
		return titleCase(parts[0]), "Holon"
	}
	return titleCase(parts[0]), titleCase(strings.Join(parts[1:], "-"))
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func appendStringField(b *strings.Builder, indent, name, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	fmt.Fprintf(b, "%s%s: %s\n", indent, name, strconv.Quote(trimmed))
}

func appendStringSliceField(b *strings.Builder, indent, name string, values []string) {
	values = compactStrings(values)
	if len(values) == 0 {
		return
	}

	b.WriteString(indent)
	b.WriteString(name)
	b.WriteString(": [")
	for i, value := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(value))
	}
	b.WriteString("]\n")
}

func appendRepeatedSkills(b *strings.Builder, skills []skill) {
	for _, skill := range skills {
		b.WriteString("  skills: {\n")
		appendStringField(b, "    ", "name", skill.Name)
		appendStringField(b, "    ", "description", skill.Description)
		appendStringField(b, "    ", "when", skill.When)
		appendStringSliceField(b, "    ", "steps", skill.Steps)
		b.WriteString("  }\n")
	}
}

func appendRepeatedSequences(b *strings.Builder, sequences []sequence) {
	for _, sequence := range sequences {
		b.WriteString("  sequences: {\n")
		appendStringField(b, "    ", "name", sequence.Name)
		appendStringField(b, "    ", "description", sequence.Description)
		for _, param := range sequence.Params {
			b.WriteString("    params: {\n")
			appendStringField(b, "      ", "name", param.Name)
			appendStringField(b, "      ", "description", param.Description)
			if param.Required {
				b.WriteString("      required: true\n")
			}
			appendStringField(b, "      ", "default", param.Default)
			b.WriteString("    }\n")
		}
		appendStringSliceField(b, "    ", "steps", sequence.Steps)
		b.WriteString("  }\n")
	}
}

func appendContract(b *strings.Builder, contract *contract) {
	if contract == nil {
		return
	}
	hasDetails := strings.TrimSpace(contract.Proto) != "" ||
		strings.TrimSpace(contract.Service) != "" ||
		len(compactStrings(contract.RPCs)) > 0
	if !hasDetails && !contract.GRPC {
		return
	}

	b.WriteString("  contract: {\n")
	appendStringField(b, "    ", "proto", contract.Proto)
	appendStringField(b, "    ", "service", contract.Service)
	appendStringSliceField(b, "    ", "rpcs", contract.RPCs)
	b.WriteString("  }\n")
}

func appendBuild(b *strings.Builder, build build) {
	if strings.TrimSpace(build.Runner) == "" &&
		strings.TrimSpace(build.Main) == "" &&
		build.Defaults == nil &&
		len(build.Members) == 0 &&
		len(build.Targets) == 0 {
		return
	}

	b.WriteString("  build: {\n")
	appendStringField(b, "    ", "runner", build.Runner)
	appendStringField(b, "    ", "main", build.Main)
	if build.Defaults != nil {
		b.WriteString("    defaults: {\n")
		appendStringField(b, "      ", "target", build.Defaults.Target)
		appendStringField(b, "      ", "mode", build.Defaults.Mode)
		b.WriteString("    }\n")
	}
	for _, member := range build.Members {
		b.WriteString("    members: {\n")
		appendStringField(b, "      ", "id", member.ID)
		appendStringField(b, "      ", "path", member.Path)
		appendStringField(b, "      ", "type", member.Type)
		b.WriteString("    }\n")
	}
	if len(build.Targets) > 0 {
		keys := make([]string, 0, len(build.Targets))
		for key := range build.Targets {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString("    targets: {\n")
			appendStringField(b, "      ", "key", key)
			b.WriteString("      value: {\n")
			for _, step := range build.Targets[key].Steps {
				appendStep(b, "        ", step)
			}
			b.WriteString("      }\n")
			b.WriteString("    }\n")
		}
	}
	b.WriteString("  }\n")
}

func appendStep(b *strings.Builder, indent string, step step) {
	b.WriteString(indent)
	b.WriteString("steps: {\n")
	if trimmed := strings.TrimSpace(step.BuildMember); trimmed != "" {
		appendStringField(b, indent+"  ", "build_member", trimmed)
	}
	if step.Exec != nil {
		b.WriteString(indent + "  exec: {\n")
		appendStringField(b, indent+"    ", "cwd", step.Exec.Cwd)
		appendStringSliceField(b, indent+"    ", "argv", step.Exec.Argv)
		b.WriteString(indent + "  }\n")
	}
	if step.Copy != nil {
		b.WriteString(indent + "  copy: {\n")
		appendStringField(b, indent+"    ", "from", step.Copy.From)
		appendStringField(b, indent+"    ", "to", step.Copy.To)
		b.WriteString(indent + "  }\n")
	}
	if step.AssertFile != nil {
		b.WriteString(indent + "  assert_file: {\n")
		appendStringField(b, indent+"    ", "path", step.AssertFile.Path)
		b.WriteString(indent + "  }\n")
	}
	if step.CopyArtifact != nil {
		b.WriteString(indent + "  copy_artifact: {\n")
		appendStringField(b, indent+"    ", "from", step.CopyArtifact.From)
		appendStringField(b, indent+"    ", "to", step.CopyArtifact.To)
		b.WriteString(indent + "  }\n")
	}
	b.WriteString(indent)
	b.WriteString("}\n")
}

func appendRequires(b *strings.Builder, requires requires, delegates delegates) {
	commands := compactStrings(append(append([]string(nil), requires.Commands...), delegates.Commands...))
	files := compactStrings(requires.Files)
	if len(commands) == 0 && len(files) == 0 {
		return
	}

	b.WriteString("  requires: {\n")
	appendStringSliceField(b, "    ", "commands", commands)
	appendStringSliceField(b, "    ", "files", files)
	b.WriteString("  }\n")
}

func appendArtifacts(b *strings.Builder, artifacts artifacts) {
	if strings.TrimSpace(artifacts.Binary) == "" && strings.TrimSpace(artifacts.Primary) == "" {
		return
	}

	b.WriteString("  artifacts: {\n")
	appendStringField(b, "    ", "binary", artifacts.Binary)
	appendStringField(b, "    ", "primary", artifacts.Primary)
	b.WriteString("  }\n")
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
