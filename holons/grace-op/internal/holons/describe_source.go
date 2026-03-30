package holons

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	godescribe "github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/grace-op/internal/progress"
	"google.golang.org/protobuf/proto"
)

const (
	describeTemplatePrefix = "describe."
	describeTemplateSuffix = ".tmpl"
	describeOutputPrefix   = "describe_generated."
)

func generateDescribeSource(manifest *LoadedManifest, reporter progress.Reporter) (restore func(), err error) {
	restore = func() {}

	if manifest == nil {
		return restore, nil
	}

	lang := strings.TrimSpace(manifest.Manifest.Lang)
	if lang == "" {
		return restore, nil
	}

	templatePath, ext, err := findDescribeTemplate(manifest.Dir, lang)
	if err != nil {
		return restore, err
	}
	if templatePath == "" {
		return restore, nil
	}

	response, err := godescribe.BuildResponse(describeProtoDir(manifest), manifest.Path)
	if err != nil {
		return restore, fmt.Errorf("build describe response: %w", err)
	}

	outputPath := filepath.Join(manifest.Dir, "gen", describeOutputPrefix+ext)
	restore, err = writeDescribeSource(templatePath, outputPath, response)
	if err != nil {
		return func() {}, err
	}

	reporter.Step(fmt.Sprintf("incode description: %s", workspaceRelativePath(outputPath)))
	return restore, nil
}

func describeProtoDir(manifest *LoadedManifest) string {
	if manifest == nil {
		return ""
	}

	candidate := filepath.Join(manifest.Dir, "proto")
	info, err := os.Stat(candidate)
	if err == nil && info.IsDir() {
		return candidate
	}

	return manifest.Dir
}

func findDescribeTemplate(holonDir, lang string) (path string, ext string, err error) {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return "", "", nil
	}

	for current := filepath.Clean(holonDir); ; {
		templateDir := filepath.Join(current, "sdk", lang+"-holons", "templates")
		path, ext, err = findDescribeTemplateInDir(templateDir)
		if err != nil {
			return "", "", err
		}
		if path != "" {
			return path, ext, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", "", nil
		}
		current = parent
	}
}

func findDescribeTemplateInDir(templateDir string) (path string, ext string, err error) {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("read template dir %s: %w", templateDir, err)
	}

	matches := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, describeTemplatePrefix) || !strings.HasSuffix(name, describeTemplateSuffix) {
			continue
		}
		matches = append(matches, name)
	}

	if len(matches) == 0 {
		return "", "", nil
	}

	sort.Strings(matches)
	if len(matches) > 1 {
		return "", "", fmt.Errorf("multiple describe templates found in %s: %s", templateDir, strings.Join(matches, ", "))
	}

	name := matches[0]
	ext = strings.TrimSuffix(strings.TrimPrefix(name, describeTemplatePrefix), describeTemplateSuffix)
	if ext == "" {
		return "", "", fmt.Errorf("describe template %s has empty extension", filepath.Join(templateDir, name))
	}
	return filepath.Join(templateDir, name), ext, nil
}

func writeDescribeSource(templatePath, outputPath string, response *holonsv1.DescribeResponse) (restore func(), err error) {
	restore = func() {}

	originalExists := false
	var original []byte
	mode := os.FileMode(0o644)

	if info, statErr := os.Stat(outputPath); statErr == nil {
		originalExists = true
		mode = info.Mode()
		original, err = os.ReadFile(outputPath)
		if err != nil {
			return func() {}, fmt.Errorf("read existing output %s: %w", outputPath, err)
		}
	} else if !os.IsNotExist(statErr) {
		return func() {}, fmt.Errorf("stat output %s: %w", outputPath, statErr)
	}

	restore = func() {
		if originalExists {
			_ = os.WriteFile(outputPath, original, mode)
			return
		}
		_ = os.Remove(outputPath)
	}

	rendered, err := renderDescribeTemplate(templatePath, outputPath, response)
	if err != nil {
		restore()
		return func() {}, err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		restore()
		return func() {}, fmt.Errorf("create output dir for %s: %w", outputPath, err)
	}
	if err := os.WriteFile(outputPath, rendered, mode); err != nil {
		restore()
		return func() {}, fmt.Errorf("write %s: %w", outputPath, err)
	}

	return restore, nil
}

func renderDescribeTemplate(templatePath, outputPath string, response *holonsv1.DescribeResponse) ([]byte, error) {
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", templatePath, err)
	}

	ext := strings.TrimPrefix(filepath.Ext(outputPath), ".")
	tmpl, err := template.New(filepath.Base(templatePath)).Funcs(describeTemplateFuncs(ext)).Parse(string(templateBytes))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, response); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", templatePath, err)
	}

	rendered := buf.Bytes()
	if ext == "go" {
		rendered, err = format.Source(rendered)
		if err != nil {
			return nil, fmt.Errorf("format generated Go source for %s: %w", outputPath, err)
		}
	}

	return rendered, nil
}

func describeTemplateFuncs(ext string) template.FuncMap {
	funcs := template.FuncMap{
		"protoBase64": func(message proto.Message) (string, error) {
			if message == nil {
				return "", nil
			}
			data, err := proto.Marshal(message)
			if err != nil {
				return "", fmt.Errorf("marshal proto message: %w", err)
			}
			return base64.StdEncoding.EncodeToString(data), nil
		},
	}
	if ext == "go" {
		funcs["goDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return goDescribeResponseLiteral(response)
		}
	}
	if ext == "rs" {
		funcs["rustDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return rustDescribeResponseLiteral(response)
		}
	}
	if ext == "c" {
		funcs["cDescribeSource"] = func(response *holonsv1.DescribeResponse) string {
			return cDescribeSource(response)
		}
	}
	if ext == "h" || ext == "hh" || ext == "hpp" || ext == "cc" || ext == "cpp" || ext == "cxx" {
		funcs["cppDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return cppDescribeResponse(response)
		}
	}
	return funcs
}

func goDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "&holonsv1.DescribeResponse{}"
	}
	return literalValue(0, func(buf *strings.Builder, indent int) {
		writeDescribeResponseLiteral(buf, response, indent)
	})
}

func writeDescribeResponseLiteral(buf *strings.Builder, response *holonsv1.DescribeResponse, indent int) {
	writeLine(buf, indent, "&holonsv1.DescribeResponse{")
	if response.GetManifest() != nil {
		buf.WriteString(goIndent(indent + 1))
		buf.WriteString("Manifest: ")
		buf.WriteString(literalValue(indent+1, func(buf *strings.Builder, indent int) {
			writeHolonManifestLiteral(buf, response.GetManifest(), indent)
		}))
		buf.WriteString(",\n")
	}
	if len(response.GetServices()) > 0 {
		writeLine(buf, indent+1, "Services: []*holonsv1.ServiceDoc{")
		for _, service := range response.GetServices() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeServiceDocLiteral(buf, service, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	writeLine(buf, indent, "}")
}

func writeHolonManifestLiteral(buf *strings.Builder, manifest *holonsv1.HolonManifest, indent int) {
	if manifest == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.HolonManifest{")
	if manifest.GetIdentity() != nil {
		buf.WriteString(goIndent(indent + 1))
		buf.WriteString("Identity: ")
		buf.WriteString(literalValue(indent+1, func(buf *strings.Builder, indent int) {
			writeIdentityLiteral(buf, manifest.GetIdentity(), indent)
		}))
		buf.WriteString(",\n")
	}
	writeStringField(buf, indent+1, "Description", manifest.GetDescription())
	writeStringField(buf, indent+1, "Lang", manifest.GetLang())
	if len(manifest.GetSkills()) > 0 {
		writeLine(buf, indent+1, "Skills: []*holonsv1.HolonManifest_Skill{")
		for _, skill := range manifest.GetSkills() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeSkillLiteral(buf, skill, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	writeStringField(buf, indent+1, "Kind", manifest.GetKind())
	if manifest.GetBuild() != nil {
		buf.WriteString(goIndent(indent + 1))
		buf.WriteString("Build: ")
		buf.WriteString(literalValue(indent+1, func(buf *strings.Builder, indent int) {
			writeBuildLiteral(buf, manifest.GetBuild(), indent)
		}))
		buf.WriteString(",\n")
	}
	if manifest.GetRequires() != nil {
		buf.WriteString(goIndent(indent + 1))
		buf.WriteString("Requires: ")
		buf.WriteString(literalValue(indent+1, func(buf *strings.Builder, indent int) {
			writeRequiresLiteral(buf, manifest.GetRequires(), indent)
		}))
		buf.WriteString(",\n")
	}
	if manifest.GetArtifacts() != nil {
		buf.WriteString(goIndent(indent + 1))
		buf.WriteString("Artifacts: ")
		buf.WriteString(literalValue(indent+1, func(buf *strings.Builder, indent int) {
			writeArtifactsLiteral(buf, manifest.GetArtifacts(), indent)
		}))
		buf.WriteString(",\n")
	}
	if len(manifest.GetSequences()) > 0 {
		writeLine(buf, indent+1, "Sequences: []*holonsv1.HolonManifest_Sequence{")
		for _, sequence := range manifest.GetSequences() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeSequenceLiteral(buf, sequence, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	writeLine(buf, indent, "}")
}

func writeIdentityLiteral(buf *strings.Builder, identity *holonsv1.HolonManifest_Identity, indent int) {
	if identity == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.HolonManifest_Identity{")
	writeStringField(buf, indent+1, "Schema", identity.GetSchema())
	writeStringField(buf, indent+1, "Uuid", identity.GetUuid())
	writeStringField(buf, indent+1, "GivenName", identity.GetGivenName())
	writeStringField(buf, indent+1, "FamilyName", identity.GetFamilyName())
	writeStringField(buf, indent+1, "Motto", identity.GetMotto())
	writeStringField(buf, indent+1, "Composer", identity.GetComposer())
	writeStringField(buf, indent+1, "Status", identity.GetStatus())
	writeStringField(buf, indent+1, "Born", identity.GetBorn())
	writeStringField(buf, indent+1, "Version", identity.GetVersion())
	writeStringSliceField(buf, indent+1, "Aliases", identity.GetAliases())
	writeLine(buf, indent, "}")
}

func writeSkillLiteral(buf *strings.Builder, skill *holonsv1.HolonManifest_Skill, indent int) {
	if skill == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.HolonManifest_Skill{")
	writeStringField(buf, indent+1, "Name", skill.GetName())
	writeStringField(buf, indent+1, "Description", skill.GetDescription())
	writeStringField(buf, indent+1, "When", skill.GetWhen())
	writeStringSliceField(buf, indent+1, "Steps", skill.GetSteps())
	writeLine(buf, indent, "}")
}

func writeSequenceLiteral(buf *strings.Builder, sequence *holonsv1.HolonManifest_Sequence, indent int) {
	if sequence == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.HolonManifest_Sequence{")
	writeStringField(buf, indent+1, "Name", sequence.GetName())
	writeStringField(buf, indent+1, "Description", sequence.GetDescription())
	if len(sequence.GetParams()) > 0 {
		writeLine(buf, indent+1, "Params: []*holonsv1.HolonManifest_Sequence_Param{")
		for _, param := range sequence.GetParams() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeSequenceParamLiteral(buf, param, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	writeStringSliceField(buf, indent+1, "Steps", sequence.GetSteps())
	writeLine(buf, indent, "}")
}

func writeSequenceParamLiteral(buf *strings.Builder, param *holonsv1.HolonManifest_Sequence_Param, indent int) {
	if param == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.HolonManifest_Sequence_Param{")
	writeStringField(buf, indent+1, "Name", param.GetName())
	writeStringField(buf, indent+1, "Description", param.GetDescription())
	writeBoolField(buf, indent+1, "Required", param.GetRequired())
	writeStringField(buf, indent+1, "Default", param.GetDefault())
	writeLine(buf, indent, "}")
}

func writeBuildLiteral(buf *strings.Builder, build *holonsv1.HolonManifest_Build, indent int) {
	if build == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.HolonManifest_Build{")
	writeStringField(buf, indent+1, "Runner", build.GetRunner())
	writeStringField(buf, indent+1, "Main", build.GetMain())
	writeLine(buf, indent, "}")
}

func writeRequiresLiteral(buf *strings.Builder, requires *holonsv1.HolonManifest_Requires, indent int) {
	if requires == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.HolonManifest_Requires{")
	writeStringSliceField(buf, indent+1, "Commands", requires.GetCommands())
	writeStringSliceField(buf, indent+1, "Files", requires.GetFiles())
	writeStringSliceField(buf, indent+1, "Platforms", requires.GetPlatforms())
	writeLine(buf, indent, "}")
}

func writeArtifactsLiteral(buf *strings.Builder, artifacts *holonsv1.HolonManifest_Artifacts, indent int) {
	if artifacts == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.HolonManifest_Artifacts{")
	writeStringField(buf, indent+1, "Binary", artifacts.GetBinary())
	writeStringField(buf, indent+1, "Primary", artifacts.GetPrimary())
	writeLine(buf, indent, "}")
}

func writeServiceDocLiteral(buf *strings.Builder, service *holonsv1.ServiceDoc, indent int) {
	if service == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.ServiceDoc{")
	writeStringField(buf, indent+1, "Name", service.GetName())
	writeStringField(buf, indent+1, "Description", service.GetDescription())
	if len(service.GetMethods()) > 0 {
		writeLine(buf, indent+1, "Methods: []*holonsv1.MethodDoc{")
		for _, method := range service.GetMethods() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeMethodDocLiteral(buf, method, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	writeLine(buf, indent, "}")
}

func writeMethodDocLiteral(buf *strings.Builder, method *holonsv1.MethodDoc, indent int) {
	if method == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.MethodDoc{")
	writeStringField(buf, indent+1, "Name", method.GetName())
	writeStringField(buf, indent+1, "Description", method.GetDescription())
	writeStringField(buf, indent+1, "InputType", method.GetInputType())
	writeStringField(buf, indent+1, "OutputType", method.GetOutputType())
	if len(method.GetInputFields()) > 0 {
		writeLine(buf, indent+1, "InputFields: []*holonsv1.FieldDoc{")
		for _, field := range method.GetInputFields() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeFieldDocLiteral(buf, field, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	if len(method.GetOutputFields()) > 0 {
		writeLine(buf, indent+1, "OutputFields: []*holonsv1.FieldDoc{")
		for _, field := range method.GetOutputFields() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeFieldDocLiteral(buf, field, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	writeBoolField(buf, indent+1, "ClientStreaming", method.GetClientStreaming())
	writeBoolField(buf, indent+1, "ServerStreaming", method.GetServerStreaming())
	writeStringField(buf, indent+1, "ExampleInput", method.GetExampleInput())
	writeLine(buf, indent, "}")
}

func writeFieldDocLiteral(buf *strings.Builder, field *holonsv1.FieldDoc, indent int) {
	if field == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.FieldDoc{")
	writeStringField(buf, indent+1, "Name", field.GetName())
	writeStringField(buf, indent+1, "Type", field.GetType())
	writeInt32Field(buf, indent+1, "Number", field.GetNumber())
	writeStringField(buf, indent+1, "Description", field.GetDescription())
	writeFieldLabelField(buf, indent+1, "Label", field.GetLabel())
	writeStringField(buf, indent+1, "MapKeyType", field.GetMapKeyType())
	writeStringField(buf, indent+1, "MapValueType", field.GetMapValueType())
	if len(field.GetNestedFields()) > 0 {
		writeLine(buf, indent+1, "NestedFields: []*holonsv1.FieldDoc{")
		for _, nested := range field.GetNestedFields() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeFieldDocLiteral(buf, nested, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	if len(field.GetEnumValues()) > 0 {
		writeLine(buf, indent+1, "EnumValues: []*holonsv1.EnumValueDoc{")
		for _, value := range field.GetEnumValues() {
			buf.WriteString(literalValue(indent+2, func(buf *strings.Builder, indent int) {
				writeEnumValueDocLiteral(buf, value, indent)
			}))
			buf.WriteString(",\n")
		}
		writeLine(buf, indent+1, "},")
	}
	writeBoolField(buf, indent+1, "Required", field.GetRequired())
	writeStringField(buf, indent+1, "Example", field.GetExample())
	writeLine(buf, indent, "}")
}

func writeEnumValueDocLiteral(buf *strings.Builder, value *holonsv1.EnumValueDoc, indent int) {
	if value == nil {
		buf.WriteString("nil")
		return
	}

	writeLine(buf, indent, "&holonsv1.EnumValueDoc{")
	writeStringField(buf, indent+1, "Name", value.GetName())
	writeInt32Field(buf, indent+1, "Number", value.GetNumber())
	writeStringField(buf, indent+1, "Description", value.GetDescription())
	writeLine(buf, indent, "}")
}

func writeStringField(buf *strings.Builder, indent int, fieldName, value string) {
	if value == "" {
		return
	}
	writeLine(buf, indent, fmt.Sprintf("%s: %s,", fieldName, strconv.Quote(value)))
}

func writeStringSliceField(buf *strings.Builder, indent int, fieldName string, values []string) {
	if len(values) == 0 {
		return
	}

	writeLine(buf, indent, fieldName+": []string{")
	for _, value := range values {
		writeLine(buf, indent+1, strconv.Quote(value)+",")
	}
	writeLine(buf, indent, "},")
}

func writeBoolField(buf *strings.Builder, indent int, fieldName string, value bool) {
	if !value {
		return
	}
	writeLine(buf, indent, fmt.Sprintf("%s: true,", fieldName))
}

func writeInt32Field(buf *strings.Builder, indent int, fieldName string, value int32) {
	if value == 0 {
		return
	}
	writeLine(buf, indent, fmt.Sprintf("%s: %d,", fieldName, value))
}

func writeFieldLabelField(buf *strings.Builder, indent int, fieldName string, value holonsv1.FieldLabel) {
	writeLine(buf, indent, fmt.Sprintf("%s: %s,", fieldName, goFieldLabelLiteral(value)))
}

func goFieldLabelLiteral(value holonsv1.FieldLabel) string {
	name := value.String()
	if strings.HasPrefix(name, "FIELD_LABEL_") {
		return "holonsv1.FieldLabel_" + name
	}
	return fmt.Sprintf("holonsv1.FieldLabel(%d)", value)
}

func literalValue(indent int, write func(*strings.Builder, int)) string {
	var buf strings.Builder
	write(&buf, indent)
	return strings.TrimSuffix(buf.String(), "\n")
}

func writeLine(buf *strings.Builder, indent int, line string) {
	buf.WriteString(goIndent(indent))
	buf.WriteString(line)
	buf.WriteString("\n")
}

func goIndent(indent int) string {
	if indent <= 0 {
		return ""
	}
	return strings.Repeat("\t", indent)
}

func rustDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "DescribeResponse {\n    manifest: None,\n    services: vec![],\n}"
	}
	return rustLiteralValue(0, func(buf *strings.Builder, indent int) {
		writeRustDescribeResponseLiteral(buf, response, indent)
	})
}

func writeRustDescribeResponseLiteral(buf *strings.Builder, response *holonsv1.DescribeResponse, indent int) {
	writeRustLine(buf, indent, "DescribeResponse {")
	if response.GetManifest() != nil {
		writeRustOptionField(buf, indent+1, "manifest", func(buf *strings.Builder, indent int) {
			writeRustHolonManifestLiteral(buf, response.GetManifest(), indent)
		})
	} else {
		writeRustLine(buf, indent+1, "manifest: None,")
	}
	writeRustRepeatedField(buf, indent+1, "services", len(response.GetServices()), func(index int, buf *strings.Builder, indent int) {
		writeRustServiceDocLiteral(buf, response.GetServices()[index], indent)
	})
	writeRustLine(buf, indent, "}")
}

func writeRustHolonManifestLiteral(buf *strings.Builder, manifest *holonsv1.HolonManifest, indent int) {
	if manifest == nil {
		buf.WriteString("None")
		return
	}

	writeRustLine(buf, indent, "HolonManifest {")
	if manifest.GetIdentity() != nil {
		writeRustOptionField(buf, indent+1, "identity", func(buf *strings.Builder, indent int) {
			writeRustIdentityLiteral(buf, manifest.GetIdentity(), indent)
		})
	} else {
		writeRustLine(buf, indent+1, "identity: None,")
	}
	writeRustStringField(buf, indent+1, "description", manifest.GetDescription())
	writeRustStringField(buf, indent+1, "lang", manifest.GetLang())
	writeRustRepeatedField(buf, indent+1, "skills", len(manifest.GetSkills()), func(index int, buf *strings.Builder, indent int) {
		writeRustSkillLiteral(buf, manifest.GetSkills()[index], indent)
	})
	writeRustLine(buf, indent+1, "contract: None,")
	writeRustStringField(buf, indent+1, "kind", manifest.GetKind())
	writeRustStringVecField(buf, indent+1, "platforms", manifest.GetPlatforms())
	writeRustStringField(buf, indent+1, "transport", manifest.GetTransport())
	if manifest.GetBuild() != nil {
		writeRustOptionField(buf, indent+1, "build", func(buf *strings.Builder, indent int) {
			writeRustBuildLiteral(buf, manifest.GetBuild(), indent)
		})
	} else {
		writeRustLine(buf, indent+1, "build: None,")
	}
	if manifest.GetRequires() != nil {
		writeRustOptionField(buf, indent+1, "requires", func(buf *strings.Builder, indent int) {
			writeRustRequiresLiteral(buf, manifest.GetRequires(), indent)
		})
	} else {
		writeRustLine(buf, indent+1, "requires: None,")
	}
	if manifest.GetArtifacts() != nil {
		writeRustOptionField(buf, indent+1, "artifacts", func(buf *strings.Builder, indent int) {
			writeRustArtifactsLiteral(buf, manifest.GetArtifacts(), indent)
		})
	} else {
		writeRustLine(buf, indent+1, "artifacts: None,")
	}
	writeRustRepeatedField(buf, indent+1, "sequences", len(manifest.GetSequences()), func(index int, buf *strings.Builder, indent int) {
		writeRustSequenceLiteral(buf, manifest.GetSequences()[index], indent)
	})
	writeRustStringField(buf, indent+1, "guide", manifest.GetGuide())
	writeRustLine(buf, indent, "}")
}

func writeRustIdentityLiteral(buf *strings.Builder, identity *holonsv1.HolonManifest_Identity, indent int) {
	writeRustLine(buf, indent, "Identity {")
	writeRustStringField(buf, indent+1, "schema", identity.GetSchema())
	writeRustStringField(buf, indent+1, "uuid", identity.GetUuid())
	writeRustStringField(buf, indent+1, "given_name", identity.GetGivenName())
	writeRustStringField(buf, indent+1, "family_name", identity.GetFamilyName())
	writeRustStringField(buf, indent+1, "motto", identity.GetMotto())
	writeRustStringField(buf, indent+1, "composer", identity.GetComposer())
	writeRustStringField(buf, indent+1, "status", identity.GetStatus())
	writeRustStringField(buf, indent+1, "born", identity.GetBorn())
	writeRustStringField(buf, indent+1, "version", identity.GetVersion())
	writeRustStringVecField(buf, indent+1, "aliases", identity.GetAliases())
	writeRustLine(buf, indent, "}")
}

func writeRustSkillLiteral(buf *strings.Builder, skill *holonsv1.HolonManifest_Skill, indent int) {
	writeRustLine(buf, indent, "Skill {")
	writeRustStringField(buf, indent+1, "name", skill.GetName())
	writeRustStringField(buf, indent+1, "description", skill.GetDescription())
	writeRustStringField(buf, indent+1, "when", skill.GetWhen())
	writeRustStringVecField(buf, indent+1, "steps", skill.GetSteps())
	writeRustLine(buf, indent, "}")
}

func writeRustSequenceLiteral(buf *strings.Builder, sequence *holonsv1.HolonManifest_Sequence, indent int) {
	writeRustLine(buf, indent, "Sequence {")
	writeRustStringField(buf, indent+1, "name", sequence.GetName())
	writeRustStringField(buf, indent+1, "description", sequence.GetDescription())
	writeRustRepeatedField(buf, indent+1, "params", len(sequence.GetParams()), func(index int, buf *strings.Builder, indent int) {
		writeRustSequenceParamLiteral(buf, sequence.GetParams()[index], indent)
	})
	writeRustStringVecField(buf, indent+1, "steps", sequence.GetSteps())
	writeRustLine(buf, indent, "}")
}

func writeRustSequenceParamLiteral(buf *strings.Builder, param *holonsv1.HolonManifest_Sequence_Param, indent int) {
	writeRustLine(buf, indent, "Param {")
	writeRustStringField(buf, indent+1, "name", param.GetName())
	writeRustStringField(buf, indent+1, "description", param.GetDescription())
	writeRustBoolField(buf, indent+1, "required", param.GetRequired())
	writeRustStringField(buf, indent+1, "default", param.GetDefault())
	writeRustLine(buf, indent, "}")
}

func writeRustBuildLiteral(buf *strings.Builder, build *holonsv1.HolonManifest_Build, indent int) {
	writeRustLine(buf, indent, "Build {")
	writeRustStringField(buf, indent+1, "runner", build.GetRunner())
	writeRustStringField(buf, indent+1, "main", build.GetMain())
	writeRustLine(buf, indent+1, "defaults: None,")
	writeRustRepeatedField(buf, indent+1, "members", 0, func(_ int, _ *strings.Builder, _ int) {})
	writeRustLine(buf, indent+1, "targets: ::std::collections::HashMap::new(),")
	writeRustStringVecField(buf, indent+1, "templates", build.GetTemplates())
	writeRustLine(buf, indent, "}")
}

func writeRustRequiresLiteral(buf *strings.Builder, requires *holonsv1.HolonManifest_Requires, indent int) {
	writeRustLine(buf, indent, "Requires {")
	writeRustStringVecField(buf, indent+1, "commands", requires.GetCommands())
	writeRustStringVecField(buf, indent+1, "files", requires.GetFiles())
	writeRustStringVecField(buf, indent+1, "platforms", requires.GetPlatforms())
	writeRustLine(buf, indent, "}")
}

func writeRustArtifactsLiteral(buf *strings.Builder, artifacts *holonsv1.HolonManifest_Artifacts, indent int) {
	writeRustLine(buf, indent, "Artifacts {")
	writeRustStringField(buf, indent+1, "binary", artifacts.GetBinary())
	writeRustStringField(buf, indent+1, "primary", artifacts.GetPrimary())
	writeRustLine(buf, indent+1, "by_target: ::std::collections::HashMap::new(),")
	writeRustLine(buf, indent, "}")
}

func writeRustServiceDocLiteral(buf *strings.Builder, service *holonsv1.ServiceDoc, indent int) {
	writeRustLine(buf, indent, "ServiceDoc {")
	writeRustStringField(buf, indent+1, "name", service.GetName())
	writeRustStringField(buf, indent+1, "description", service.GetDescription())
	writeRustRepeatedField(buf, indent+1, "methods", len(service.GetMethods()), func(index int, buf *strings.Builder, indent int) {
		writeRustMethodDocLiteral(buf, service.GetMethods()[index], indent)
	})
	writeRustLine(buf, indent, "}")
}

func writeRustMethodDocLiteral(buf *strings.Builder, method *holonsv1.MethodDoc, indent int) {
	writeRustLine(buf, indent, "MethodDoc {")
	writeRustStringField(buf, indent+1, "name", method.GetName())
	writeRustStringField(buf, indent+1, "description", method.GetDescription())
	writeRustStringField(buf, indent+1, "input_type", method.GetInputType())
	writeRustStringField(buf, indent+1, "output_type", method.GetOutputType())
	writeRustRepeatedField(buf, indent+1, "input_fields", len(method.GetInputFields()), func(index int, buf *strings.Builder, indent int) {
		writeRustFieldDocLiteral(buf, method.GetInputFields()[index], indent)
	})
	writeRustRepeatedField(buf, indent+1, "output_fields", len(method.GetOutputFields()), func(index int, buf *strings.Builder, indent int) {
		writeRustFieldDocLiteral(buf, method.GetOutputFields()[index], indent)
	})
	writeRustBoolField(buf, indent+1, "client_streaming", method.GetClientStreaming())
	writeRustBoolField(buf, indent+1, "server_streaming", method.GetServerStreaming())
	writeRustStringField(buf, indent+1, "example_input", method.GetExampleInput())
	writeRustLine(buf, indent, "}")
}

func writeRustFieldDocLiteral(buf *strings.Builder, field *holonsv1.FieldDoc, indent int) {
	writeRustLine(buf, indent, "FieldDoc {")
	writeRustStringField(buf, indent+1, "name", field.GetName())
	writeRustStringField(buf, indent+1, "r#type", field.GetType())
	writeRustInt32Field(buf, indent+1, "number", field.GetNumber())
	writeRustStringField(buf, indent+1, "description", field.GetDescription())
	writeRustLine(buf, indent+1, fmt.Sprintf("label: %s as i32,", rustFieldLabelLiteral(field.GetLabel())))
	writeRustStringField(buf, indent+1, "map_key_type", field.GetMapKeyType())
	writeRustStringField(buf, indent+1, "map_value_type", field.GetMapValueType())
	writeRustRepeatedField(buf, indent+1, "nested_fields", len(field.GetNestedFields()), func(index int, buf *strings.Builder, indent int) {
		writeRustFieldDocLiteral(buf, field.GetNestedFields()[index], indent)
	})
	writeRustRepeatedField(buf, indent+1, "enum_values", len(field.GetEnumValues()), func(index int, buf *strings.Builder, indent int) {
		writeRustEnumValueDocLiteral(buf, field.GetEnumValues()[index], indent)
	})
	writeRustBoolField(buf, indent+1, "required", field.GetRequired())
	writeRustStringField(buf, indent+1, "example", field.GetExample())
	writeRustLine(buf, indent, "}")
}

func writeRustEnumValueDocLiteral(buf *strings.Builder, value *holonsv1.EnumValueDoc, indent int) {
	writeRustLine(buf, indent, "EnumValueDoc {")
	writeRustStringField(buf, indent+1, "name", value.GetName())
	writeRustInt32Field(buf, indent+1, "number", value.GetNumber())
	writeRustStringField(buf, indent+1, "description", value.GetDescription())
	writeRustLine(buf, indent, "}")
}

func writeRustOptionField(buf *strings.Builder, indent int, fieldName string, write func(*strings.Builder, int)) {
	writeRustLine(buf, indent, fieldName+": Some(")
	write(buf, indent+1)
	writeRustLine(buf, indent, "),")
}

func writeRustRepeatedField(buf *strings.Builder, indent int, fieldName string, count int, write func(int, *strings.Builder, int)) {
	if count == 0 {
		writeRustLine(buf, indent, fieldName+": vec![],")
		return
	}

	writeRustLine(buf, indent, fieldName+": vec![")
	for index := 0; index < count; index++ {
		write(index, buf, indent+1)
		writeRustLine(buf, indent+1, ",")
	}
	writeRustLine(buf, indent, "],")
}

func writeRustStringField(buf *strings.Builder, indent int, fieldName, value string) {
	writeRustLine(buf, indent, fmt.Sprintf("%s: %s.to_string(),", fieldName, strconv.Quote(value)))
}

func writeRustStringVecField(buf *strings.Builder, indent int, fieldName string, values []string) {
	if len(values) == 0 {
		writeRustLine(buf, indent, fieldName+": vec![],")
		return
	}

	writeRustLine(buf, indent, fieldName+": vec![")
	for _, value := range values {
		writeRustLine(buf, indent+1, fmt.Sprintf("%s.to_string(),", strconv.Quote(value)))
	}
	writeRustLine(buf, indent, "],")
}

func writeRustBoolField(buf *strings.Builder, indent int, fieldName string, value bool) {
	if value {
		writeRustLine(buf, indent, fmt.Sprintf("%s: true,", fieldName))
		return
	}
	writeRustLine(buf, indent, fmt.Sprintf("%s: false,", fieldName))
}

func writeRustInt32Field(buf *strings.Builder, indent int, fieldName string, value int32) {
	writeRustLine(buf, indent, fmt.Sprintf("%s: %d,", fieldName, value))
}

func rustFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_UNSPECIFIED:
		return "FieldLabel::Unspecified"
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return "FieldLabel::Optional"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return "FieldLabel::Repeated"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return "FieldLabel::Map"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return "FieldLabel::Required"
	default:
		return fmt.Sprintf("FieldLabel::from_i32(%d).unwrap_or(FieldLabel::Unspecified)", value)
	}
}

func rustLiteralValue(indent int, write func(*strings.Builder, int)) string {
	var buf strings.Builder
	write(&buf, indent)
	return strings.TrimSuffix(buf.String(), "\n")
}

func writeRustLine(buf *strings.Builder, indent int, line string) {
	buf.WriteString(rustIndent(indent))
	buf.WriteString(line)
	buf.WriteString("\n")
}

func rustIndent(indent int) string {
	if indent <= 0 {
		return ""
	}
	return strings.Repeat("    ", indent)
}

type cDescribeEmitter struct {
	counter     int
	definitions []string
}

func cDescribeSource(response *holonsv1.DescribeResponse) string {
	emitter := &cDescribeEmitter{}
	servicesName, serviceCount := emitter.writeServiceArray(response.GetServices())

	var buf strings.Builder
	for _, definition := range emitter.definitions {
		buf.WriteString(definition)
		buf.WriteString("\n\n")
	}

	writeCLine(&buf, 0, "static holons_describe_response_t holons_generated_describe_response_value = {")
	buf.WriteString(cManifestInitializer(response.GetManifest(), 1))
	writeCLine(&buf, 1, fmt.Sprintf(".services = %s,", cArrayPointer(servicesName)))
	writeCLine(&buf, 1, fmt.Sprintf(".service_count = %d,", serviceCount))
	writeCLine(&buf, 0, "};")
	buf.WriteString("\n")
	writeCLine(&buf, 0, "const holons_describe_response_t *holons_generated_describe_response(void) {")
	writeCLine(&buf, 1, "return &holons_generated_describe_response_value;")
	writeCLine(&buf, 0, "}")

	return strings.TrimSuffix(buf.String(), "\n")
}

func (e *cDescribeEmitter) nextName(prefix string) string {
	e.counter++
	return fmt.Sprintf("holons_generated_%s_%d", prefix, e.counter)
}

func (e *cDescribeEmitter) writeServiceArray(services []*holonsv1.ServiceDoc) (string, int) {
	if len(services) == 0 {
		return "", 0
	}

	name := e.nextName("services")
	items := make([]string, 0, len(services))
	for _, service := range services {
		methodsName, methodCount := e.writeMethodArray(service.GetMethods())
		items = append(items, cServiceInitializer(service, methodsName, methodCount))
	}
	e.writeDefinition("holons_service_doc_t", name, items)
	return name, len(services)
}

func (e *cDescribeEmitter) writeMethodArray(methods []*holonsv1.MethodDoc) (string, int) {
	if len(methods) == 0 {
		return "", 0
	}

	name := e.nextName("methods")
	items := make([]string, 0, len(methods))
	for _, method := range methods {
		inputFieldsName, inputFieldCount := e.writeFieldArray(method.GetInputFields())
		outputFieldsName, outputFieldCount := e.writeFieldArray(method.GetOutputFields())
		items = append(items, cMethodInitializer(method, inputFieldsName, inputFieldCount, outputFieldsName, outputFieldCount))
	}
	e.writeDefinition("holons_method_doc_t", name, items)
	return name, len(methods)
}

func (e *cDescribeEmitter) writeFieldArray(fields []*holonsv1.FieldDoc) (string, int) {
	if len(fields) == 0 {
		return "", 0
	}

	name := e.nextName("fields")
	items := make([]string, 0, len(fields))
	for _, field := range fields {
		nestedFieldsName, nestedFieldCount := e.writeFieldArray(field.GetNestedFields())
		enumValuesName, enumValueCount := e.writeEnumValueArray(field.GetEnumValues())
		items = append(items, cFieldInitializer(field, nestedFieldsName, nestedFieldCount, enumValuesName, enumValueCount))
	}
	e.writeDefinition("holons_field_doc_t", name, items)
	return name, len(fields)
}

func (e *cDescribeEmitter) writeEnumValueArray(values []*holonsv1.EnumValueDoc) (string, int) {
	if len(values) == 0 {
		return "", 0
	}

	name := e.nextName("enum_values")
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, cEnumValueInitializer(value))
	}
	e.writeDefinition("holons_enum_value_doc_t", name, items)
	return name, len(values)
}

func (e *cDescribeEmitter) writeDefinition(typeName, name string, items []string) {
	var buf strings.Builder
	writeCLine(&buf, 0, fmt.Sprintf("static %s %s[] = {", typeName, name))
	for i, item := range items {
		buf.WriteString(indentCLiteral(item, 1))
		if i < len(items)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	writeCLine(&buf, 0, "};")
	e.definitions = append(e.definitions, strings.TrimSuffix(buf.String(), "\n"))
}

func cManifestInitializer(manifest *holonsv1.HolonManifest, indent int) string {
	var buf strings.Builder
	writeCLine(&buf, indent, ".manifest = {")
	if manifest != nil {
		writeCLine(&buf, indent+1, ".identity = {")
		if identity := manifest.GetIdentity(); identity != nil {
			writeCStringField(&buf, indent+2, ".uuid", identity.GetUuid())
			writeCStringField(&buf, indent+2, ".given_name", identity.GetGivenName())
			writeCStringField(&buf, indent+2, ".family_name", identity.GetFamilyName())
			writeCStringField(&buf, indent+2, ".motto", identity.GetMotto())
			writeCStringField(&buf, indent+2, ".composer", identity.GetComposer())
			writeCStringField(&buf, indent+2, ".status", identity.GetStatus())
			writeCStringField(&buf, indent+2, ".born", identity.GetBorn())
		}
		writeCLine(&buf, indent+1, "},")
		writeCStringField(&buf, indent+1, ".lang", manifest.GetLang())
		writeCStringField(&buf, indent+1, ".kind", manifest.GetKind())
		if build := manifest.GetBuild(); build != nil {
			writeCLine(&buf, indent+1, ".build = {")
			writeCStringField(&buf, indent+2, ".runner", build.GetRunner())
			writeCStringField(&buf, indent+2, ".main", build.GetMain())
			writeCLine(&buf, indent+1, "},")
		}
		if artifacts := manifest.GetArtifacts(); artifacts != nil {
			writeCLine(&buf, indent+1, ".artifacts = {")
			writeCStringField(&buf, indent+2, ".binary", artifacts.GetBinary())
			writeCStringField(&buf, indent+2, ".primary", artifacts.GetPrimary())
			writeCLine(&buf, indent+1, "},")
		}
	}
	writeCLine(&buf, indent, "},")
	return buf.String()
}

func cServiceInitializer(service *holonsv1.ServiceDoc, methodsName string, methodCount int) string {
	var buf strings.Builder
	writeCLine(&buf, 0, "{")
	writeCStringField(&buf, 1, ".name", service.GetName())
	writeCStringField(&buf, 1, ".description", service.GetDescription())
	writeCLine(&buf, 1, fmt.Sprintf(".methods = %s,", cArrayPointer(methodsName)))
	writeCLine(&buf, 1, fmt.Sprintf(".method_count = %d,", methodCount))
	writeCLine(&buf, 0, "}")
	return strings.TrimSuffix(buf.String(), "\n")
}

func cMethodInitializer(method *holonsv1.MethodDoc, inputFieldsName string, inputFieldCount int, outputFieldsName string, outputFieldCount int) string {
	var buf strings.Builder
	writeCLine(&buf, 0, "{")
	writeCStringField(&buf, 1, ".name", method.GetName())
	writeCStringField(&buf, 1, ".description", method.GetDescription())
	writeCStringField(&buf, 1, ".input_type", method.GetInputType())
	writeCStringField(&buf, 1, ".output_type", method.GetOutputType())
	writeCLine(&buf, 1, fmt.Sprintf(".input_fields = %s,", cArrayPointer(inputFieldsName)))
	writeCLine(&buf, 1, fmt.Sprintf(".input_field_count = %d,", inputFieldCount))
	writeCLine(&buf, 1, fmt.Sprintf(".output_fields = %s,", cArrayPointer(outputFieldsName)))
	writeCLine(&buf, 1, fmt.Sprintf(".output_field_count = %d,", outputFieldCount))
	writeCLine(&buf, 1, fmt.Sprintf(".client_streaming = %d,", cBool(method.GetClientStreaming())))
	writeCLine(&buf, 1, fmt.Sprintf(".server_streaming = %d,", cBool(method.GetServerStreaming())))
	writeCStringField(&buf, 1, ".example_input", method.GetExampleInput())
	writeCLine(&buf, 0, "}")
	return strings.TrimSuffix(buf.String(), "\n")
}

func cFieldInitializer(field *holonsv1.FieldDoc, nestedFieldsName string, nestedFieldCount int, enumValuesName string, enumValueCount int) string {
	var buf strings.Builder
	writeCLine(&buf, 0, "{")
	writeCStringField(&buf, 1, ".name", field.GetName())
	writeCStringField(&buf, 1, ".type", field.GetType())
	writeCLine(&buf, 1, fmt.Sprintf(".number = %d,", field.GetNumber()))
	writeCStringField(&buf, 1, ".description", field.GetDescription())
	writeCLine(&buf, 1, fmt.Sprintf(".label = %s,", cFieldLabelLiteral(field.GetLabel())))
	writeCStringField(&buf, 1, ".map_key_type", field.GetMapKeyType())
	writeCStringField(&buf, 1, ".map_value_type", field.GetMapValueType())
	writeCLine(&buf, 1, fmt.Sprintf(".nested_fields = %s,", cArrayPointer(nestedFieldsName)))
	writeCLine(&buf, 1, fmt.Sprintf(".nested_field_count = %d,", nestedFieldCount))
	writeCLine(&buf, 1, fmt.Sprintf(".enum_values = %s,", cArrayPointer(enumValuesName)))
	writeCLine(&buf, 1, fmt.Sprintf(".enum_value_count = %d,", enumValueCount))
	writeCLine(&buf, 1, fmt.Sprintf(".required = %d,", cBool(field.GetRequired())))
	writeCStringField(&buf, 1, ".example", field.GetExample())
	writeCLine(&buf, 0, "}")
	return strings.TrimSuffix(buf.String(), "\n")
}

func cEnumValueInitializer(value *holonsv1.EnumValueDoc) string {
	var buf strings.Builder
	writeCLine(&buf, 0, "{")
	writeCStringField(&buf, 1, ".name", value.GetName())
	writeCLine(&buf, 1, fmt.Sprintf(".number = %d,", value.GetNumber()))
	writeCStringField(&buf, 1, ".description", value.GetDescription())
	writeCLine(&buf, 0, "}")
	return strings.TrimSuffix(buf.String(), "\n")
}

func cFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return "HOLONS_FIELD_LABEL_OPTIONAL"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return "HOLONS_FIELD_LABEL_REPEATED"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return "HOLONS_FIELD_LABEL_MAP"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return "HOLONS_FIELD_LABEL_REQUIRED"
	default:
		return "HOLONS_FIELD_LABEL_UNSPECIFIED"
	}
}

func cArrayPointer(name string) string {
	if name == "" {
		return "NULL"
	}
	return name
}

func cBool(value bool) int {
	if value {
		return 1
	}
	return 0
}

func writeCStringField(buf *strings.Builder, indent int, fieldName, value string) {
	if value == "" {
		return
	}
	writeCLine(buf, indent, fmt.Sprintf("%s = %s,", fieldName, strconv.Quote(value)))
}

func writeCLine(buf *strings.Builder, indent int, line string) {
	buf.WriteString(cIndent(indent))
	buf.WriteString(line)
	buf.WriteString("\n")
}

func cIndent(indent int) string {
	if indent <= 0 {
		return ""
	}
	return strings.Repeat("  ", indent)
}

func indentCLiteral(literal string, indent int) string {
	lines := strings.Split(literal, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = cIndent(indent) + line
	}
	return strings.Join(lines, "\n")
}

func cppDescribeResponse(response *holonsv1.DescribeResponse) string {
	var buf strings.Builder
	writeCLine(&buf, 0, "[] {")
	writeCLine(&buf, 1, "holons::v1::DescribeResponse response;")
	if response != nil {
		if response.GetManifest() != nil {
			writeCPPHolonManifest(&buf, response.GetManifest())
		}
		for _, service := range response.GetServices() {
			writeCPPServiceDoc(&buf, service)
		}
	}
	writeCLine(&buf, 1, "return response;")
	writeCLine(&buf, 0, "}()")
	return strings.TrimSuffix(buf.String(), "\n")
}

func writeCPPHolonManifest(buf *strings.Builder, manifest *holonsv1.HolonManifest) {
	if manifest == nil {
		return
	}

	writeCLine(buf, 1, "{")
	writeCLine(buf, 2, "auto *manifest = response.mutable_manifest();")
	if identity := manifest.GetIdentity(); identity != nil {
		writeCLine(buf, 2, "{")
		writeCLine(buf, 3, "auto *identity = manifest->mutable_identity();")
		writeCPPStringSetter(buf, 3, "identity", "set_schema", identity.GetSchema())
		writeCPPStringSetter(buf, 3, "identity", "set_uuid", identity.GetUuid())
		writeCPPStringSetter(buf, 3, "identity", "set_given_name", identity.GetGivenName())
		writeCPPStringSetter(buf, 3, "identity", "set_family_name", identity.GetFamilyName())
		writeCPPStringSetter(buf, 3, "identity", "set_motto", identity.GetMotto())
		writeCPPStringSetter(buf, 3, "identity", "set_composer", identity.GetComposer())
		writeCPPStringSetter(buf, 3, "identity", "set_status", identity.GetStatus())
		writeCPPStringSetter(buf, 3, "identity", "set_born", identity.GetBorn())
		writeCPPStringSetter(buf, 3, "identity", "set_version", identity.GetVersion())
		for _, alias := range identity.GetAliases() {
			writeCPPStringAdder(buf, 3, "identity", "add_aliases", alias)
		}
		writeCLine(buf, 2, "}")
	}
	writeCPPStringSetter(buf, 2, "manifest", "set_description", manifest.GetDescription())
	writeCPPStringSetter(buf, 2, "manifest", "set_lang", manifest.GetLang())
	for _, skill := range manifest.GetSkills() {
		writeCLine(buf, 2, "{")
		writeCLine(buf, 3, "auto *skill = manifest->add_skills();")
		writeCPPStringSetter(buf, 3, "skill", "set_name", skill.GetName())
		writeCPPStringSetter(buf, 3, "skill", "set_description", skill.GetDescription())
		writeCPPStringSetter(buf, 3, "skill", "set_when", skill.GetWhen())
		for _, step := range skill.GetSteps() {
			writeCPPStringAdder(buf, 3, "skill", "add_steps", step)
		}
		writeCLine(buf, 2, "}")
	}
	writeCPPStringSetter(buf, 2, "manifest", "set_kind", manifest.GetKind())
	if build := manifest.GetBuild(); build != nil {
		writeCLine(buf, 2, "{")
		writeCLine(buf, 3, "auto *build = manifest->mutable_build();")
		writeCPPStringSetter(buf, 3, "build", "set_runner", build.GetRunner())
		writeCPPStringSetter(buf, 3, "build", "set_main", build.GetMain())
		writeCLine(buf, 2, "}")
	}
	if requires := manifest.GetRequires(); requires != nil {
		writeCLine(buf, 2, "{")
		writeCLine(buf, 3, "auto *manifest_requires = manifest->mutable_requires_();")
		for _, command := range requires.GetCommands() {
			writeCPPStringAdder(buf, 3, "manifest_requires", "add_commands", command)
		}
		for _, file := range requires.GetFiles() {
			writeCPPStringAdder(buf, 3, "manifest_requires", "add_files", file)
		}
		for _, platform := range requires.GetPlatforms() {
			writeCPPStringAdder(buf, 3, "manifest_requires", "add_platforms", platform)
		}
		writeCLine(buf, 2, "}")
	}
	if artifacts := manifest.GetArtifacts(); artifacts != nil {
		writeCLine(buf, 2, "{")
		writeCLine(buf, 3, "auto *artifacts = manifest->mutable_artifacts();")
		writeCPPStringSetter(buf, 3, "artifacts", "set_binary", artifacts.GetBinary())
		writeCPPStringSetter(buf, 3, "artifacts", "set_primary", artifacts.GetPrimary())
		writeCLine(buf, 2, "}")
	}
	for _, sequence := range manifest.GetSequences() {
		writeCLine(buf, 2, "{")
		writeCLine(buf, 3, "auto *sequence = manifest->add_sequences();")
		writeCPPStringSetter(buf, 3, "sequence", "set_name", sequence.GetName())
		writeCPPStringSetter(buf, 3, "sequence", "set_description", sequence.GetDescription())
		for _, param := range sequence.GetParams() {
			writeCLine(buf, 3, "{")
			writeCLine(buf, 4, "auto *param = sequence->add_params();")
			writeCPPStringSetter(buf, 4, "param", "set_name", param.GetName())
			writeCPPStringSetter(buf, 4, "param", "set_description", param.GetDescription())
			writeCPPBoolSetter(buf, 4, "param", "set_required", param.GetRequired())
			writeCPPStringSetter(buf, 4, "param", "set_default", param.GetDefault())
			writeCLine(buf, 3, "}")
		}
		for _, step := range sequence.GetSteps() {
			writeCPPStringAdder(buf, 3, "sequence", "add_steps", step)
		}
		writeCLine(buf, 2, "}")
	}
	writeCLine(buf, 1, "}")
}

func writeCPPServiceDoc(buf *strings.Builder, service *holonsv1.ServiceDoc) {
	if service == nil {
		return
	}

	writeCLine(buf, 1, "{")
	writeCLine(buf, 2, "auto *service = response.add_services();")
	writeCPPStringSetter(buf, 2, "service", "set_name", service.GetName())
	writeCPPStringSetter(buf, 2, "service", "set_description", service.GetDescription())
	for _, method := range service.GetMethods() {
		writeCLine(buf, 2, "{")
		writeCLine(buf, 3, "auto *method = service->add_methods();")
		writeCPPStringSetter(buf, 3, "method", "set_name", method.GetName())
		writeCPPStringSetter(buf, 3, "method", "set_description", method.GetDescription())
		writeCPPStringSetter(buf, 3, "method", "set_input_type", method.GetInputType())
		writeCPPStringSetter(buf, 3, "method", "set_output_type", method.GetOutputType())
		for _, field := range method.GetInputFields() {
			writeCPPFieldDoc(buf, 3, "method->add_input_fields()", field)
		}
		for _, field := range method.GetOutputFields() {
			writeCPPFieldDoc(buf, 3, "method->add_output_fields()", field)
		}
		writeCPPBoolSetter(buf, 3, "method", "set_client_streaming", method.GetClientStreaming())
		writeCPPBoolSetter(buf, 3, "method", "set_server_streaming", method.GetServerStreaming())
		writeCPPStringSetter(buf, 3, "method", "set_example_input", method.GetExampleInput())
		writeCLine(buf, 2, "}")
	}
	writeCLine(buf, 1, "}")
}

func writeCPPFieldDoc(buf *strings.Builder, indent int, target string, field *holonsv1.FieldDoc) {
	if field == nil {
		return
	}

	writeCLine(buf, indent, "{")
	writeCLine(buf, indent+1, "auto *field = "+target+";")
	writeCPPStringSetter(buf, indent+1, "field", "set_name", field.GetName())
	writeCPPStringSetter(buf, indent+1, "field", "set_type", field.GetType())
	writeCPPInt32Setter(buf, indent+1, "field", "set_number", field.GetNumber())
	writeCPPStringSetter(buf, indent+1, "field", "set_description", field.GetDescription())
	writeCLine(buf, indent+1, fmt.Sprintf("field->set_label(static_cast<holons::v1::FieldLabel>(%d));", field.GetLabel()))
	writeCPPStringSetter(buf, indent+1, "field", "set_map_key_type", field.GetMapKeyType())
	writeCPPStringSetter(buf, indent+1, "field", "set_map_value_type", field.GetMapValueType())
	for _, nested := range field.GetNestedFields() {
		writeCPPFieldDoc(buf, indent+1, "field->add_nested_fields()", nested)
	}
	for _, value := range field.GetEnumValues() {
		writeCLine(buf, indent+1, "{")
		writeCLine(buf, indent+2, "auto *value = field->add_enum_values();")
		writeCPPStringSetter(buf, indent+2, "value", "set_name", value.GetName())
		writeCPPInt32Setter(buf, indent+2, "value", "set_number", value.GetNumber())
		writeCPPStringSetter(buf, indent+2, "value", "set_description", value.GetDescription())
		writeCLine(buf, indent+1, "}")
	}
	writeCPPBoolSetter(buf, indent+1, "field", "set_required", field.GetRequired())
	writeCPPStringSetter(buf, indent+1, "field", "set_example", field.GetExample())
	writeCLine(buf, indent, "}")
}

func writeCPPStringSetter(buf *strings.Builder, indent int, target, setter, value string) {
	if value == "" {
		return
	}
	writeCLine(buf, indent, fmt.Sprintf("%s->%s(%s);", target, setter, strconv.Quote(value)))
}

func writeCPPStringAdder(buf *strings.Builder, indent int, target, adder, value string) {
	writeCLine(buf, indent, fmt.Sprintf("%s->%s(%s);", target, adder, strconv.Quote(value)))
}

func writeCPPBoolSetter(buf *strings.Builder, indent int, target, setter string, value bool) {
	if !value {
		return
	}
	writeCLine(buf, indent, fmt.Sprintf("%s->%s(true);", target, setter))
}

func writeCPPInt32Setter(buf *strings.Builder, indent int, target, setter string, value int32) {
	if value == 0 {
		return
	}
	writeCLine(buf, indent, fmt.Sprintf("%s->%s(%d);", target, setter, value))
}
