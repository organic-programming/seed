package holons

import (
	"bytes"
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

	response, err := buildDescribeResponse(manifest)
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

func buildDescribeResponse(manifest *LoadedManifest) (*holonsv1.DescribeResponse, error) {
	candidates := describeProtoCandidates(manifest)
	var firstSuccess *holonsv1.DescribeResponse
	var firstErr error
	for _, candidate := range candidates {
		response, err := godescribe.BuildResponse(candidate, manifest.Path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if firstSuccess == nil {
			firstSuccess = response
		}
		if len(response.GetServices()) > 0 {
			return response, nil
		}
	}
	if firstSuccess != nil {
		return firstSuccess, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fmt.Errorf("no proto sources found for describe generation")
}

func describeProtoCandidates(manifest *LoadedManifest) []string {
	if manifest == nil {
		return nil
	}

	var candidates []string
	addCandidate := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		for _, existing := range candidates {
			if existing == trimmed {
				return
			}
		}
		candidates = append(candidates, trimmed)
	}

	manifestProtoDir := filepath.Dir(strings.TrimSpace(manifest.Path))
	if describeDirHasProto(manifestProtoDir) {
		addCandidate(manifestProtoDir)
	}

	candidate := filepath.Join(manifest.Dir, "proto")
	info, err := os.Stat(candidate)
	if err == nil && info.IsDir() {
		if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
			if resolvedInfo, resolvedErr := os.Stat(resolved); resolvedErr == nil && resolvedInfo.IsDir() {
				addCandidate(resolved)
			}
		}
		addCandidate(candidate)
	}

	addCandidate(manifest.Dir)
	return candidates
}

func describeDirHasProto(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".proto") {
			return true
		}
	}
	return false
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
	funcs := template.FuncMap{}
	if ext == "go" {
		funcs["goDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return goDescribeResponseLiteral(response)
		}
	}
	if ext == "dart" {
		funcs["dartDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return dartDescribeResponseLiteral(response)
		}
	}
	if ext == "py" {
		funcs["pythonDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return pythonDescribeResponseLiteral(response)
		}
	}
	if ext == "js" {
		funcs["jsDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return jsDescribeResponseLiteral(response)
		}
	}
	if ext == "swift" {
		funcs["swiftDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return swiftDescribeResponseLiteral(response)
		}
	}
	if ext == "cs" {
		funcs["csharpDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return csharpDescribeResponseLiteral(response)
		}
	}
	if ext == "java" {
		funcs["javaDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return javaDescribeResponseLiteral(response)
		}
	}
	if ext == "kt" {
		funcs["kotlinDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return kotlinDescribeResponseLiteral(response)
		}
	}
	if ext == "rb" {
		funcs["rubyDescribeResponse"] = func(response *holonsv1.DescribeResponse) string {
			return rubyDescribeResponseLiteral(response)
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
	describeBytes, _ := proto.Marshal(response)
	describeArrayLen := len(describeBytes)
	if describeArrayLen == 0 {
		describeArrayLen = 1
	}

	var buf strings.Builder
	for _, definition := range emitter.definitions {
		buf.WriteString(definition)
		buf.WriteString("\n\n")
	}

	writeCLine(&buf, 0, fmt.Sprintf("static const unsigned char holons_generated_describe_response_bytes_value[%d] = {", describeArrayLen))
	if len(describeBytes) == 0 {
		writeCLine(&buf, 1, "0x00,")
	} else {
		for _, b := range describeBytes {
			writeCLine(&buf, 1, fmt.Sprintf("0x%02x,", b))
		}
	}
	writeCLine(&buf, 0, "};")
	buf.WriteString("\n")
	writeCLine(&buf, 0, "static holons_describe_response_t holons_generated_describe_response_value = {")
	buf.WriteString(cManifestInitializer(response.GetManifest(), 1))
	writeCLine(&buf, 1, fmt.Sprintf(".services = %s,", cArrayPointer(servicesName)))
	writeCLine(&buf, 1, fmt.Sprintf(".service_count = %d,", serviceCount))
	writeCLine(&buf, 0, "};")
	buf.WriteString("\n")
	writeCLine(&buf, 0, "const unsigned char *holons_generated_describe_response_bytes(size_t *len) {")
	writeCLine(&buf, 1, "if (len != NULL) {")
	writeCLine(&buf, 2, fmt.Sprintf("*len = %d;", len(describeBytes)))
	writeCLine(&buf, 1, "}")
	writeCLine(&buf, 1, "return holons_generated_describe_response_bytes_value;")
	writeCLine(&buf, 0, "}")
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

	fieldVar := fmt.Sprintf("field_%d", indent)

	writeCLine(buf, indent, "{")
	writeCLine(buf, indent+1, "auto *"+fieldVar+" = "+target+";")
	writeCPPStringSetter(buf, indent+1, fieldVar, "set_name", field.GetName())
	writeCPPStringSetter(buf, indent+1, fieldVar, "set_type", field.GetType())
	writeCPPInt32Setter(buf, indent+1, fieldVar, "set_number", field.GetNumber())
	writeCPPStringSetter(buf, indent+1, fieldVar, "set_description", field.GetDescription())
	writeCLine(buf, indent+1, fmt.Sprintf("%s->set_label(static_cast<holons::v1::FieldLabel>(%d));", fieldVar, field.GetLabel()))
	writeCPPStringSetter(buf, indent+1, fieldVar, "set_map_key_type", field.GetMapKeyType())
	writeCPPStringSetter(buf, indent+1, fieldVar, "set_map_value_type", field.GetMapValueType())
	for _, nested := range field.GetNestedFields() {
		writeCPPFieldDoc(buf, indent+1, fieldVar+"->add_nested_fields()", nested)
	}
	for _, value := range field.GetEnumValues() {
		writeCLine(buf, indent+1, "{")
		writeCLine(buf, indent+2, "auto *value = "+fieldVar+"->add_enum_values();")
		writeCPPStringSetter(buf, indent+2, "value", "set_name", value.GetName())
		writeCPPInt32Setter(buf, indent+2, "value", "set_number", value.GetNumber())
		writeCPPStringSetter(buf, indent+2, "value", "set_description", value.GetDescription())
		writeCLine(buf, indent+1, "}")
	}
	writeCPPBoolSetter(buf, indent+1, fieldVar, "set_required", field.GetRequired())
	writeCPPStringSetter(buf, indent+1, fieldVar, "set_example", field.GetExample())
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

type generatedField struct {
	name  string
	value string
}

func sourceStringLiteral(value string) string {
	var buf strings.Builder
	buf.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			buf.WriteString(`\\`)
		case '"':
			buf.WriteString(`\"`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		default:
			if r < 0x20 {
				buf.WriteString(fmt.Sprintf(`\u%04X`, r))
				continue
			}
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
	return buf.String()
}

func genericIndent(unit string, indent int) string {
	if indent <= 0 {
		return ""
	}
	return strings.Repeat(unit, indent)
}

func dartDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "DescribeResponse()"
	}
	return dartDescribeResponseExpr(response, 0)
}

func dartDescribeResponseExpr(response *holonsv1.DescribeResponse, indent int) string {
	fields := make([]generatedField, 0, 2)
	if response.GetManifest() != nil {
		fields = append(fields, generatedField{
			name:  "manifest",
			value: dartHolonManifestExpr(response.GetManifest(), indent+1),
		})
	}
	if len(response.GetServices()) > 0 {
		values := make([]string, 0, len(response.GetServices()))
		for _, service := range response.GetServices() {
			values = append(values, dartServiceDocExpr(service, indent+2))
		}
		fields = append(fields, generatedField{
			name:  "services",
			value: dartListExpr(indent+1, values),
		})
	}
	return dartCallExpr(indent, "DescribeResponse", fields)
}

func dartHolonManifestExpr(manifest *holonsv1.HolonManifest, indent int) string {
	if manifest == nil {
		return "HolonManifest()"
	}
	fields := make([]generatedField, 0, 11)
	if manifest.GetIdentity() != nil {
		fields = append(fields, generatedField{
			name:  "identity",
			value: dartIdentityExpr(manifest.GetIdentity(), indent+1),
		})
	}
	if manifest.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(manifest.GetDescription())})
	}
	if manifest.GetLang() != "" {
		fields = append(fields, generatedField{name: "lang", value: sourceStringLiteral(manifest.GetLang())})
	}
	if len(manifest.GetSkills()) > 0 {
		values := make([]string, 0, len(manifest.GetSkills()))
		for _, skill := range manifest.GetSkills() {
			values = append(values, dartSkillExpr(skill, indent+2))
		}
		fields = append(fields, generatedField{name: "skills", value: dartListExpr(indent+1, values)})
	}
	if manifest.GetKind() != "" {
		fields = append(fields, generatedField{name: "kind", value: sourceStringLiteral(manifest.GetKind())})
	}
	if len(manifest.GetPlatforms()) > 0 {
		values := make([]string, 0, len(manifest.GetPlatforms()))
		for _, platform := range manifest.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "platforms", value: dartListExpr(indent+1, values)})
	}
	if manifest.GetTransport() != "" {
		fields = append(fields, generatedField{name: "transport", value: sourceStringLiteral(manifest.GetTransport())})
	}
	if manifest.GetBuild() != nil {
		fields = append(fields, generatedField{name: "build", value: dartBuildExpr(manifest.GetBuild(), indent+1)})
	}
	if manifest.GetRequires() != nil {
		fields = append(fields, generatedField{name: "requires", value: dartRequiresExpr(manifest.GetRequires(), indent+1)})
	}
	if manifest.GetArtifacts() != nil {
		fields = append(fields, generatedField{name: "artifacts", value: dartArtifactsExpr(manifest.GetArtifacts(), indent+1)})
	}
	if len(manifest.GetSequences()) > 0 {
		values := make([]string, 0, len(manifest.GetSequences()))
		for _, sequence := range manifest.GetSequences() {
			values = append(values, dartSequenceExpr(sequence, indent+2))
		}
		fields = append(fields, generatedField{name: "sequences", value: dartListExpr(indent+1, values)})
	}
	if manifest.GetGuide() != "" {
		fields = append(fields, generatedField{name: "guide", value: sourceStringLiteral(manifest.GetGuide())})
	}
	return dartCallExpr(indent, "HolonManifest", fields)
}

func dartIdentityExpr(identity *holonsv1.HolonManifest_Identity, indent int) string {
	if identity == nil {
		return "HolonManifest_Identity()"
	}
	fields := make([]generatedField, 0, 10)
	if identity.GetSchema() != "" {
		fields = append(fields, generatedField{name: "schema", value: sourceStringLiteral(identity.GetSchema())})
	}
	if identity.GetUuid() != "" {
		fields = append(fields, generatedField{name: "uuid", value: sourceStringLiteral(identity.GetUuid())})
	}
	if identity.GetGivenName() != "" {
		fields = append(fields, generatedField{name: "givenName", value: sourceStringLiteral(identity.GetGivenName())})
	}
	if identity.GetFamilyName() != "" {
		fields = append(fields, generatedField{name: "familyName", value: sourceStringLiteral(identity.GetFamilyName())})
	}
	if identity.GetMotto() != "" {
		fields = append(fields, generatedField{name: "motto", value: sourceStringLiteral(identity.GetMotto())})
	}
	if identity.GetComposer() != "" {
		fields = append(fields, generatedField{name: "composer", value: sourceStringLiteral(identity.GetComposer())})
	}
	if identity.GetStatus() != "" {
		fields = append(fields, generatedField{name: "status", value: sourceStringLiteral(identity.GetStatus())})
	}
	if identity.GetBorn() != "" {
		fields = append(fields, generatedField{name: "born", value: sourceStringLiteral(identity.GetBorn())})
	}
	if identity.GetVersion() != "" {
		fields = append(fields, generatedField{name: "version", value: sourceStringLiteral(identity.GetVersion())})
	}
	if len(identity.GetAliases()) > 0 {
		values := make([]string, 0, len(identity.GetAliases()))
		for _, alias := range identity.GetAliases() {
			values = append(values, sourceStringLiteral(alias))
		}
		fields = append(fields, generatedField{name: "aliases", value: dartListExpr(indent+1, values)})
	}
	return dartCallExpr(indent, "HolonManifest_Identity", fields)
}

func dartSkillExpr(skill *holonsv1.HolonManifest_Skill, indent int) string {
	if skill == nil {
		return "HolonManifest_Skill()"
	}
	fields := make([]generatedField, 0, 4)
	if skill.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(skill.GetName())})
	}
	if skill.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(skill.GetDescription())})
	}
	if skill.GetWhen() != "" {
		fields = append(fields, generatedField{name: "when", value: sourceStringLiteral(skill.GetWhen())})
	}
	if len(skill.GetSteps()) > 0 {
		values := make([]string, 0, len(skill.GetSteps()))
		for _, step := range skill.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "steps", value: dartListExpr(indent+1, values)})
	}
	return dartCallExpr(indent, "HolonManifest_Skill", fields)
}

func dartSequenceExpr(sequence *holonsv1.HolonManifest_Sequence, indent int) string {
	if sequence == nil {
		return "HolonManifest_Sequence()"
	}
	fields := make([]generatedField, 0, 4)
	if sequence.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(sequence.GetName())})
	}
	if sequence.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(sequence.GetDescription())})
	}
	if len(sequence.GetParams()) > 0 {
		values := make([]string, 0, len(sequence.GetParams()))
		for _, param := range sequence.GetParams() {
			values = append(values, dartSequenceParamExpr(param, indent+2))
		}
		fields = append(fields, generatedField{name: "params", value: dartListExpr(indent+1, values)})
	}
	if len(sequence.GetSteps()) > 0 {
		values := make([]string, 0, len(sequence.GetSteps()))
		for _, step := range sequence.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "steps", value: dartListExpr(indent+1, values)})
	}
	return dartCallExpr(indent, "HolonManifest_Sequence", fields)
}

func dartSequenceParamExpr(param *holonsv1.HolonManifest_Sequence_Param, indent int) string {
	if param == nil {
		return "HolonManifest_Sequence_Param()"
	}
	fields := make([]generatedField, 0, 4)
	if param.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(param.GetName())})
	}
	if param.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(param.GetDescription())})
	}
	if param.GetRequired() {
		fields = append(fields, generatedField{name: "required", value: "true"})
	}
	if param.GetDefault() != "" {
		fields = append(fields, generatedField{name: "default_4", value: sourceStringLiteral(param.GetDefault())})
	}
	return dartCallExpr(indent, "HolonManifest_Sequence_Param", fields)
}

func dartBuildExpr(build *holonsv1.HolonManifest_Build, indent int) string {
	if build == nil {
		return "HolonManifest_Build()"
	}
	fields := make([]generatedField, 0, 3)
	if build.GetRunner() != "" {
		fields = append(fields, generatedField{name: "runner", value: sourceStringLiteral(build.GetRunner())})
	}
	if build.GetMain() != "" {
		fields = append(fields, generatedField{name: "main", value: sourceStringLiteral(build.GetMain())})
	}
	if len(build.GetTemplates()) > 0 {
		values := make([]string, 0, len(build.GetTemplates()))
		for _, template := range build.GetTemplates() {
			values = append(values, sourceStringLiteral(template))
		}
		fields = append(fields, generatedField{name: "templates", value: dartListExpr(indent+1, values)})
	}
	return dartCallExpr(indent, "HolonManifest_Build", fields)
}

func dartRequiresExpr(requires *holonsv1.HolonManifest_Requires, indent int) string {
	if requires == nil {
		return "HolonManifest_Requires()"
	}
	fields := make([]generatedField, 0, 3)
	if len(requires.GetCommands()) > 0 {
		values := make([]string, 0, len(requires.GetCommands()))
		for _, command := range requires.GetCommands() {
			values = append(values, sourceStringLiteral(command))
		}
		fields = append(fields, generatedField{name: "commands", value: dartListExpr(indent+1, values)})
	}
	if len(requires.GetFiles()) > 0 {
		values := make([]string, 0, len(requires.GetFiles()))
		for _, file := range requires.GetFiles() {
			values = append(values, sourceStringLiteral(file))
		}
		fields = append(fields, generatedField{name: "files", value: dartListExpr(indent+1, values)})
	}
	if len(requires.GetPlatforms()) > 0 {
		values := make([]string, 0, len(requires.GetPlatforms()))
		for _, platform := range requires.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "platforms", value: dartListExpr(indent+1, values)})
	}
	return dartCallExpr(indent, "HolonManifest_Requires", fields)
}

func dartArtifactsExpr(artifacts *holonsv1.HolonManifest_Artifacts, indent int) string {
	if artifacts == nil {
		return "HolonManifest_Artifacts()"
	}
	fields := make([]generatedField, 0, 2)
	if artifacts.GetBinary() != "" {
		fields = append(fields, generatedField{name: "binary", value: sourceStringLiteral(artifacts.GetBinary())})
	}
	if artifacts.GetPrimary() != "" {
		fields = append(fields, generatedField{name: "primary", value: sourceStringLiteral(artifacts.GetPrimary())})
	}
	return dartCallExpr(indent, "HolonManifest_Artifacts", fields)
}

func dartServiceDocExpr(service *holonsv1.ServiceDoc, indent int) string {
	if service == nil {
		return "ServiceDoc()"
	}
	fields := make([]generatedField, 0, 3)
	if service.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(service.GetName())})
	}
	if service.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(service.GetDescription())})
	}
	if len(service.GetMethods()) > 0 {
		values := make([]string, 0, len(service.GetMethods()))
		for _, method := range service.GetMethods() {
			values = append(values, dartMethodDocExpr(method, indent+2))
		}
		fields = append(fields, generatedField{name: "methods", value: dartListExpr(indent+1, values)})
	}
	return dartCallExpr(indent, "ServiceDoc", fields)
}

func dartMethodDocExpr(method *holonsv1.MethodDoc, indent int) string {
	if method == nil {
		return "MethodDoc()"
	}
	fields := make([]generatedField, 0, 9)
	if method.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(method.GetName())})
	}
	if method.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(method.GetDescription())})
	}
	if method.GetInputType() != "" {
		fields = append(fields, generatedField{name: "inputType", value: sourceStringLiteral(method.GetInputType())})
	}
	if method.GetOutputType() != "" {
		fields = append(fields, generatedField{name: "outputType", value: sourceStringLiteral(method.GetOutputType())})
	}
	if len(method.GetInputFields()) > 0 {
		values := make([]string, 0, len(method.GetInputFields()))
		for _, field := range method.GetInputFields() {
			values = append(values, dartFieldDocExpr(field, indent+2))
		}
		fields = append(fields, generatedField{name: "inputFields", value: dartListExpr(indent+1, values)})
	}
	if len(method.GetOutputFields()) > 0 {
		values := make([]string, 0, len(method.GetOutputFields()))
		for _, field := range method.GetOutputFields() {
			values = append(values, dartFieldDocExpr(field, indent+2))
		}
		fields = append(fields, generatedField{name: "outputFields", value: dartListExpr(indent+1, values)})
	}
	if method.GetClientStreaming() {
		fields = append(fields, generatedField{name: "clientStreaming", value: "true"})
	}
	if method.GetServerStreaming() {
		fields = append(fields, generatedField{name: "serverStreaming", value: "true"})
	}
	if method.GetExampleInput() != "" {
		fields = append(fields, generatedField{name: "exampleInput", value: sourceStringLiteral(method.GetExampleInput())})
	}
	return dartCallExpr(indent, "MethodDoc", fields)
}

func dartFieldDocExpr(field *holonsv1.FieldDoc, indent int) string {
	if field == nil {
		return "FieldDoc()"
	}
	fields := make([]generatedField, 0, 11)
	if field.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(field.GetName())})
	}
	if field.GetType() != "" {
		fields = append(fields, generatedField{name: "type", value: sourceStringLiteral(field.GetType())})
	}
	if field.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "number", value: fmt.Sprintf("%d", field.GetNumber())})
	}
	if field.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(field.GetDescription())})
	}
	fields = append(fields, generatedField{name: "label", value: dartFieldLabelLiteral(field.GetLabel())})
	if field.GetMapKeyType() != "" {
		fields = append(fields, generatedField{name: "mapKeyType", value: sourceStringLiteral(field.GetMapKeyType())})
	}
	if field.GetMapValueType() != "" {
		fields = append(fields, generatedField{name: "mapValueType", value: sourceStringLiteral(field.GetMapValueType())})
	}
	if len(field.GetNestedFields()) > 0 {
		values := make([]string, 0, len(field.GetNestedFields()))
		for _, nested := range field.GetNestedFields() {
			values = append(values, dartFieldDocExpr(nested, indent+2))
		}
		fields = append(fields, generatedField{name: "nestedFields", value: dartListExpr(indent+1, values)})
	}
	if len(field.GetEnumValues()) > 0 {
		values := make([]string, 0, len(field.GetEnumValues()))
		for _, value := range field.GetEnumValues() {
			values = append(values, dartEnumValueDocExpr(value, indent+2))
		}
		fields = append(fields, generatedField{name: "enumValues", value: dartListExpr(indent+1, values)})
	}
	if field.GetRequired() {
		fields = append(fields, generatedField{name: "required", value: "true"})
	}
	if field.GetExample() != "" {
		fields = append(fields, generatedField{name: "example", value: sourceStringLiteral(field.GetExample())})
	}
	return dartCallExpr(indent, "FieldDoc", fields)
}

func dartEnumValueDocExpr(value *holonsv1.EnumValueDoc, indent int) string {
	if value == nil {
		return "EnumValueDoc()"
	}
	fields := make([]generatedField, 0, 3)
	if value.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(value.GetName())})
	}
	if value.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "number", value: fmt.Sprintf("%d", value.GetNumber())})
	}
	if value.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(value.GetDescription())})
	}
	return dartCallExpr(indent, "EnumValueDoc", fields)
}

func dartCallExpr(indent int, name string, fields []generatedField) string {
	if len(fields) == 0 {
		return name + "()"
	}
	var buf strings.Builder
	buf.WriteString(name)
	buf.WriteString("(\n")
	for _, field := range fields {
		buf.WriteString(genericIndent("  ", indent+1))
		buf.WriteString(field.name)
		buf.WriteString(": ")
		buf.WriteString(field.value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("  ", indent))
	buf.WriteString(")")
	return buf.String()
}

func dartListExpr(indent int, values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for _, value := range values {
		buf.WriteString(genericIndent("  ", indent+1))
		buf.WriteString(value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("  ", indent))
	buf.WriteString("]")
	return buf.String()
}

func dartFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_UNSPECIFIED:
		return "FieldLabel.FIELD_LABEL_UNSPECIFIED"
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return "FieldLabel.FIELD_LABEL_OPTIONAL"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return "FieldLabel.FIELD_LABEL_REPEATED"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return "FieldLabel.FIELD_LABEL_MAP"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return "FieldLabel.FIELD_LABEL_REQUIRED"
	default:
		return fmt.Sprintf("FieldLabel.valueOf(%d)", value)
	}
}

func pythonDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "describe_pb2.DescribeResponse()"
	}
	return pythonDescribeResponseExpr(response, 0)
}

func pythonDescribeResponseExpr(response *holonsv1.DescribeResponse, indent int) string {
	fields := make([]generatedField, 0, 2)
	if response.GetManifest() != nil {
		fields = append(fields, generatedField{name: "manifest", value: pythonHolonManifestExpr(response.GetManifest(), indent+1)})
	}
	if len(response.GetServices()) > 0 {
		values := make([]string, 0, len(response.GetServices()))
		for _, service := range response.GetServices() {
			values = append(values, pythonServiceDocExpr(service, indent+2))
		}
		fields = append(fields, generatedField{name: "services", value: pythonListExpr(indent+1, values)})
	}
	return pythonCallExpr(indent, "describe_pb2.DescribeResponse", fields)
}

func pythonHolonManifestExpr(manifest *holonsv1.HolonManifest, indent int) string {
	if manifest == nil {
		return "manifest_pb2.HolonManifest()"
	}
	fields := make([]generatedField, 0, 11)
	if manifest.GetIdentity() != nil {
		fields = append(fields, generatedField{name: "identity", value: pythonIdentityExpr(manifest.GetIdentity(), indent+1)})
	}
	if manifest.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(manifest.GetDescription())})
	}
	if manifest.GetLang() != "" {
		fields = append(fields, generatedField{name: "lang", value: sourceStringLiteral(manifest.GetLang())})
	}
	if len(manifest.GetSkills()) > 0 {
		values := make([]string, 0, len(manifest.GetSkills()))
		for _, skill := range manifest.GetSkills() {
			values = append(values, pythonSkillExpr(skill, indent+2))
		}
		fields = append(fields, generatedField{name: "skills", value: pythonListExpr(indent+1, values)})
	}
	if manifest.GetKind() != "" {
		fields = append(fields, generatedField{name: "kind", value: sourceStringLiteral(manifest.GetKind())})
	}
	if len(manifest.GetPlatforms()) > 0 {
		values := make([]string, 0, len(manifest.GetPlatforms()))
		for _, platform := range manifest.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "platforms", value: pythonListExpr(indent+1, values)})
	}
	if manifest.GetTransport() != "" {
		fields = append(fields, generatedField{name: "transport", value: sourceStringLiteral(manifest.GetTransport())})
	}
	if manifest.GetBuild() != nil {
		fields = append(fields, generatedField{name: "build", value: pythonBuildExpr(manifest.GetBuild(), indent+1)})
	}
	if manifest.GetRequires() != nil {
		fields = append(fields, generatedField{name: "requires", value: pythonRequiresExpr(manifest.GetRequires(), indent+1)})
	}
	if manifest.GetArtifacts() != nil {
		fields = append(fields, generatedField{name: "artifacts", value: pythonArtifactsExpr(manifest.GetArtifacts(), indent+1)})
	}
	if len(manifest.GetSequences()) > 0 {
		values := make([]string, 0, len(manifest.GetSequences()))
		for _, sequence := range manifest.GetSequences() {
			values = append(values, pythonSequenceExpr(sequence, indent+2))
		}
		fields = append(fields, generatedField{name: "sequences", value: pythonListExpr(indent+1, values)})
	}
	if manifest.GetGuide() != "" {
		fields = append(fields, generatedField{name: "guide", value: sourceStringLiteral(manifest.GetGuide())})
	}
	return pythonCallExpr(indent, "manifest_pb2.HolonManifest", fields)
}

func pythonIdentityExpr(identity *holonsv1.HolonManifest_Identity, indent int) string {
	if identity == nil {
		return "manifest_pb2.HolonManifest.Identity()"
	}
	fields := make([]generatedField, 0, 10)
	if identity.GetSchema() != "" {
		fields = append(fields, generatedField{name: "schema", value: sourceStringLiteral(identity.GetSchema())})
	}
	if identity.GetUuid() != "" {
		fields = append(fields, generatedField{name: "uuid", value: sourceStringLiteral(identity.GetUuid())})
	}
	if identity.GetGivenName() != "" {
		fields = append(fields, generatedField{name: "given_name", value: sourceStringLiteral(identity.GetGivenName())})
	}
	if identity.GetFamilyName() != "" {
		fields = append(fields, generatedField{name: "family_name", value: sourceStringLiteral(identity.GetFamilyName())})
	}
	if identity.GetMotto() != "" {
		fields = append(fields, generatedField{name: "motto", value: sourceStringLiteral(identity.GetMotto())})
	}
	if identity.GetComposer() != "" {
		fields = append(fields, generatedField{name: "composer", value: sourceStringLiteral(identity.GetComposer())})
	}
	if identity.GetStatus() != "" {
		fields = append(fields, generatedField{name: "status", value: sourceStringLiteral(identity.GetStatus())})
	}
	if identity.GetBorn() != "" {
		fields = append(fields, generatedField{name: "born", value: sourceStringLiteral(identity.GetBorn())})
	}
	if identity.GetVersion() != "" {
		fields = append(fields, generatedField{name: "version", value: sourceStringLiteral(identity.GetVersion())})
	}
	if len(identity.GetAliases()) > 0 {
		values := make([]string, 0, len(identity.GetAliases()))
		for _, alias := range identity.GetAliases() {
			values = append(values, sourceStringLiteral(alias))
		}
		fields = append(fields, generatedField{name: "aliases", value: pythonListExpr(indent+1, values)})
	}
	return pythonCallExpr(indent, "manifest_pb2.HolonManifest.Identity", fields)
}

func pythonSkillExpr(skill *holonsv1.HolonManifest_Skill, indent int) string {
	if skill == nil {
		return "manifest_pb2.HolonManifest.Skill()"
	}
	fields := make([]generatedField, 0, 4)
	if skill.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(skill.GetName())})
	}
	if skill.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(skill.GetDescription())})
	}
	if skill.GetWhen() != "" {
		fields = append(fields, generatedField{name: "when", value: sourceStringLiteral(skill.GetWhen())})
	}
	if len(skill.GetSteps()) > 0 {
		values := make([]string, 0, len(skill.GetSteps()))
		for _, step := range skill.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "steps", value: pythonListExpr(indent+1, values)})
	}
	return pythonCallExpr(indent, "manifest_pb2.HolonManifest.Skill", fields)
}

func pythonSequenceExpr(sequence *holonsv1.HolonManifest_Sequence, indent int) string {
	if sequence == nil {
		return "manifest_pb2.HolonManifest.Sequence()"
	}
	fields := make([]generatedField, 0, 4)
	if sequence.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(sequence.GetName())})
	}
	if sequence.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(sequence.GetDescription())})
	}
	if len(sequence.GetParams()) > 0 {
		values := make([]string, 0, len(sequence.GetParams()))
		for _, param := range sequence.GetParams() {
			values = append(values, pythonSequenceParamExpr(param, indent+2))
		}
		fields = append(fields, generatedField{name: "params", value: pythonListExpr(indent+1, values)})
	}
	if len(sequence.GetSteps()) > 0 {
		values := make([]string, 0, len(sequence.GetSteps()))
		for _, step := range sequence.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "steps", value: pythonListExpr(indent+1, values)})
	}
	return pythonCallExpr(indent, "manifest_pb2.HolonManifest.Sequence", fields)
}

func pythonSequenceParamExpr(param *holonsv1.HolonManifest_Sequence_Param, indent int) string {
	if param == nil {
		return "manifest_pb2.HolonManifest.Sequence.Param()"
	}
	fields := make([]generatedField, 0, 4)
	if param.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(param.GetName())})
	}
	if param.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(param.GetDescription())})
	}
	if param.GetRequired() {
		fields = append(fields, generatedField{name: "required", value: "True"})
	}
	if param.GetDefault() != "" {
		fields = append(fields, generatedField{name: "default", value: sourceStringLiteral(param.GetDefault())})
	}
	return pythonCallExpr(indent, "manifest_pb2.HolonManifest.Sequence.Param", fields)
}

func pythonBuildExpr(build *holonsv1.HolonManifest_Build, indent int) string {
	if build == nil {
		return "manifest_pb2.HolonManifest.Build()"
	}
	fields := make([]generatedField, 0, 3)
	if build.GetRunner() != "" {
		fields = append(fields, generatedField{name: "runner", value: sourceStringLiteral(build.GetRunner())})
	}
	if build.GetMain() != "" {
		fields = append(fields, generatedField{name: "main", value: sourceStringLiteral(build.GetMain())})
	}
	if len(build.GetTemplates()) > 0 {
		values := make([]string, 0, len(build.GetTemplates()))
		for _, template := range build.GetTemplates() {
			values = append(values, sourceStringLiteral(template))
		}
		fields = append(fields, generatedField{name: "templates", value: pythonListExpr(indent+1, values)})
	}
	return pythonCallExpr(indent, "manifest_pb2.HolonManifest.Build", fields)
}

func pythonRequiresExpr(requires *holonsv1.HolonManifest_Requires, indent int) string {
	if requires == nil {
		return "manifest_pb2.HolonManifest.Requires()"
	}
	fields := make([]generatedField, 0, 3)
	if len(requires.GetCommands()) > 0 {
		values := make([]string, 0, len(requires.GetCommands()))
		for _, command := range requires.GetCommands() {
			values = append(values, sourceStringLiteral(command))
		}
		fields = append(fields, generatedField{name: "commands", value: pythonListExpr(indent+1, values)})
	}
	if len(requires.GetFiles()) > 0 {
		values := make([]string, 0, len(requires.GetFiles()))
		for _, file := range requires.GetFiles() {
			values = append(values, sourceStringLiteral(file))
		}
		fields = append(fields, generatedField{name: "files", value: pythonListExpr(indent+1, values)})
	}
	if len(requires.GetPlatforms()) > 0 {
		values := make([]string, 0, len(requires.GetPlatforms()))
		for _, platform := range requires.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "platforms", value: pythonListExpr(indent+1, values)})
	}
	return pythonCallExpr(indent, "manifest_pb2.HolonManifest.Requires", fields)
}

func pythonArtifactsExpr(artifacts *holonsv1.HolonManifest_Artifacts, indent int) string {
	if artifacts == nil {
		return "manifest_pb2.HolonManifest.Artifacts()"
	}
	fields := make([]generatedField, 0, 2)
	if artifacts.GetBinary() != "" {
		fields = append(fields, generatedField{name: "binary", value: sourceStringLiteral(artifacts.GetBinary())})
	}
	if artifacts.GetPrimary() != "" {
		fields = append(fields, generatedField{name: "primary", value: sourceStringLiteral(artifacts.GetPrimary())})
	}
	return pythonCallExpr(indent, "manifest_pb2.HolonManifest.Artifacts", fields)
}

func pythonServiceDocExpr(service *holonsv1.ServiceDoc, indent int) string {
	if service == nil {
		return "describe_pb2.ServiceDoc()"
	}
	fields := make([]generatedField, 0, 3)
	if service.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(service.GetName())})
	}
	if service.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(service.GetDescription())})
	}
	if len(service.GetMethods()) > 0 {
		values := make([]string, 0, len(service.GetMethods()))
		for _, method := range service.GetMethods() {
			values = append(values, pythonMethodDocExpr(method, indent+2))
		}
		fields = append(fields, generatedField{name: "methods", value: pythonListExpr(indent+1, values)})
	}
	return pythonCallExpr(indent, "describe_pb2.ServiceDoc", fields)
}

func pythonMethodDocExpr(method *holonsv1.MethodDoc, indent int) string {
	if method == nil {
		return "describe_pb2.MethodDoc()"
	}
	fields := make([]generatedField, 0, 9)
	if method.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(method.GetName())})
	}
	if method.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(method.GetDescription())})
	}
	if method.GetInputType() != "" {
		fields = append(fields, generatedField{name: "input_type", value: sourceStringLiteral(method.GetInputType())})
	}
	if method.GetOutputType() != "" {
		fields = append(fields, generatedField{name: "output_type", value: sourceStringLiteral(method.GetOutputType())})
	}
	if len(method.GetInputFields()) > 0 {
		values := make([]string, 0, len(method.GetInputFields()))
		for _, field := range method.GetInputFields() {
			values = append(values, pythonFieldDocExpr(field, indent+2))
		}
		fields = append(fields, generatedField{name: "input_fields", value: pythonListExpr(indent+1, values)})
	}
	if len(method.GetOutputFields()) > 0 {
		values := make([]string, 0, len(method.GetOutputFields()))
		for _, field := range method.GetOutputFields() {
			values = append(values, pythonFieldDocExpr(field, indent+2))
		}
		fields = append(fields, generatedField{name: "output_fields", value: pythonListExpr(indent+1, values)})
	}
	if method.GetClientStreaming() {
		fields = append(fields, generatedField{name: "client_streaming", value: "True"})
	}
	if method.GetServerStreaming() {
		fields = append(fields, generatedField{name: "server_streaming", value: "True"})
	}
	if method.GetExampleInput() != "" {
		fields = append(fields, generatedField{name: "example_input", value: sourceStringLiteral(method.GetExampleInput())})
	}
	return pythonCallExpr(indent, "describe_pb2.MethodDoc", fields)
}

func pythonFieldDocExpr(field *holonsv1.FieldDoc, indent int) string {
	if field == nil {
		return "describe_pb2.FieldDoc()"
	}
	fields := make([]generatedField, 0, 11)
	if field.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(field.GetName())})
	}
	if field.GetType() != "" {
		fields = append(fields, generatedField{name: "type", value: sourceStringLiteral(field.GetType())})
	}
	if field.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "number", value: fmt.Sprintf("%d", field.GetNumber())})
	}
	if field.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(field.GetDescription())})
	}
	fields = append(fields, generatedField{name: "label", value: pythonFieldLabelLiteral(field.GetLabel())})
	if field.GetMapKeyType() != "" {
		fields = append(fields, generatedField{name: "map_key_type", value: sourceStringLiteral(field.GetMapKeyType())})
	}
	if field.GetMapValueType() != "" {
		fields = append(fields, generatedField{name: "map_value_type", value: sourceStringLiteral(field.GetMapValueType())})
	}
	if len(field.GetNestedFields()) > 0 {
		values := make([]string, 0, len(field.GetNestedFields()))
		for _, nested := range field.GetNestedFields() {
			values = append(values, pythonFieldDocExpr(nested, indent+2))
		}
		fields = append(fields, generatedField{name: "nested_fields", value: pythonListExpr(indent+1, values)})
	}
	if len(field.GetEnumValues()) > 0 {
		values := make([]string, 0, len(field.GetEnumValues()))
		for _, value := range field.GetEnumValues() {
			values = append(values, pythonEnumValueDocExpr(value, indent+2))
		}
		fields = append(fields, generatedField{name: "enum_values", value: pythonListExpr(indent+1, values)})
	}
	if field.GetRequired() {
		fields = append(fields, generatedField{name: "required", value: "True"})
	}
	if field.GetExample() != "" {
		fields = append(fields, generatedField{name: "example", value: sourceStringLiteral(field.GetExample())})
	}
	return pythonCallExpr(indent, "describe_pb2.FieldDoc", fields)
}

func pythonEnumValueDocExpr(value *holonsv1.EnumValueDoc, indent int) string {
	if value == nil {
		return "describe_pb2.EnumValueDoc()"
	}
	fields := make([]generatedField, 0, 3)
	if value.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(value.GetName())})
	}
	if value.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "number", value: fmt.Sprintf("%d", value.GetNumber())})
	}
	if value.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(value.GetDescription())})
	}
	return pythonCallExpr(indent, "describe_pb2.EnumValueDoc", fields)
}

func pythonCallExpr(indent int, name string, fields []generatedField) string {
	if len(fields) == 0 {
		return name + "()"
	}
	var buf strings.Builder
	buf.WriteString(name)
	buf.WriteString("(\n")
	for _, field := range fields {
		buf.WriteString(genericIndent("    ", indent+1))
		buf.WriteString(field.name)
		buf.WriteString("=")
		buf.WriteString(field.value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("    ", indent))
	buf.WriteString(")")
	return buf.String()
}

func pythonListExpr(indent int, values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for _, value := range values {
		buf.WriteString(genericIndent("    ", indent+1))
		buf.WriteString(value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("    ", indent))
	buf.WriteString("]")
	return buf.String()
}

func pythonFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_UNSPECIFIED:
		return "describe_pb2.FIELD_LABEL_UNSPECIFIED"
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return "describe_pb2.FIELD_LABEL_OPTIONAL"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return "describe_pb2.FIELD_LABEL_REPEATED"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return "describe_pb2.FIELD_LABEL_MAP"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return "describe_pb2.FIELD_LABEL_REQUIRED"
	default:
		return fmt.Sprintf("%d", value)
	}
}

func rubyDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "::Holons::V1::DescribeResponse.new"
	}
	return rubyDescribeResponseExpr(response, 0)
}

func rubyDescribeResponseExpr(response *holonsv1.DescribeResponse, indent int) string {
	fields := make([]generatedField, 0, 2)
	if response.GetManifest() != nil {
		fields = append(fields, generatedField{name: "manifest", value: rubyHolonManifestExpr(response.GetManifest(), indent+1)})
	}
	if len(response.GetServices()) > 0 {
		values := make([]string, 0, len(response.GetServices()))
		for _, service := range response.GetServices() {
			values = append(values, rubyServiceDocExpr(service, indent+2))
		}
		fields = append(fields, generatedField{name: "services", value: rubyArrayExpr(indent+1, values)})
	}
	return rubyCallExpr(indent, "::Holons::V1::DescribeResponse.new", fields)
}

func rubyHolonManifestExpr(manifest *holonsv1.HolonManifest, indent int) string {
	if manifest == nil {
		return "::Holons::V1::HolonManifest.new"
	}
	fields := make([]generatedField, 0, 11)
	if manifest.GetIdentity() != nil {
		fields = append(fields, generatedField{name: "identity", value: rubyIdentityExpr(manifest.GetIdentity(), indent+1)})
	}
	if manifest.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(manifest.GetDescription())})
	}
	if manifest.GetLang() != "" {
		fields = append(fields, generatedField{name: "lang", value: sourceStringLiteral(manifest.GetLang())})
	}
	if len(manifest.GetSkills()) > 0 {
		values := make([]string, 0, len(manifest.GetSkills()))
		for _, skill := range manifest.GetSkills() {
			values = append(values, rubySkillExpr(skill, indent+2))
		}
		fields = append(fields, generatedField{name: "skills", value: rubyArrayExpr(indent+1, values)})
	}
	if manifest.GetKind() != "" {
		fields = append(fields, generatedField{name: "kind", value: sourceStringLiteral(manifest.GetKind())})
	}
	if len(manifest.GetPlatforms()) > 0 {
		values := make([]string, 0, len(manifest.GetPlatforms()))
		for _, platform := range manifest.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "platforms", value: rubyArrayExpr(indent+1, values)})
	}
	if manifest.GetTransport() != "" {
		fields = append(fields, generatedField{name: "transport", value: sourceStringLiteral(manifest.GetTransport())})
	}
	if manifest.GetBuild() != nil {
		fields = append(fields, generatedField{name: "build", value: rubyBuildExpr(manifest.GetBuild(), indent+1)})
	}
	if manifest.GetRequires() != nil {
		fields = append(fields, generatedField{name: "requires", value: rubyRequiresExpr(manifest.GetRequires(), indent+1)})
	}
	if manifest.GetArtifacts() != nil {
		fields = append(fields, generatedField{name: "artifacts", value: rubyArtifactsExpr(manifest.GetArtifacts(), indent+1)})
	}
	if len(manifest.GetSequences()) > 0 {
		values := make([]string, 0, len(manifest.GetSequences()))
		for _, sequence := range manifest.GetSequences() {
			values = append(values, rubySequenceExpr(sequence, indent+2))
		}
		fields = append(fields, generatedField{name: "sequences", value: rubyArrayExpr(indent+1, values)})
	}
	if manifest.GetGuide() != "" {
		fields = append(fields, generatedField{name: "guide", value: sourceStringLiteral(manifest.GetGuide())})
	}
	return rubyCallExpr(indent, "::Holons::V1::HolonManifest.new", fields)
}

func rubyIdentityExpr(identity *holonsv1.HolonManifest_Identity, indent int) string {
	if identity == nil {
		return "::Holons::V1::HolonManifest::Identity.new"
	}
	fields := make([]generatedField, 0, 10)
	if identity.GetSchema() != "" {
		fields = append(fields, generatedField{name: "schema", value: sourceStringLiteral(identity.GetSchema())})
	}
	if identity.GetUuid() != "" {
		fields = append(fields, generatedField{name: "uuid", value: sourceStringLiteral(identity.GetUuid())})
	}
	if identity.GetGivenName() != "" {
		fields = append(fields, generatedField{name: "given_name", value: sourceStringLiteral(identity.GetGivenName())})
	}
	if identity.GetFamilyName() != "" {
		fields = append(fields, generatedField{name: "family_name", value: sourceStringLiteral(identity.GetFamilyName())})
	}
	if identity.GetMotto() != "" {
		fields = append(fields, generatedField{name: "motto", value: sourceStringLiteral(identity.GetMotto())})
	}
	if identity.GetComposer() != "" {
		fields = append(fields, generatedField{name: "composer", value: sourceStringLiteral(identity.GetComposer())})
	}
	if identity.GetStatus() != "" {
		fields = append(fields, generatedField{name: "status", value: sourceStringLiteral(identity.GetStatus())})
	}
	if identity.GetBorn() != "" {
		fields = append(fields, generatedField{name: "born", value: sourceStringLiteral(identity.GetBorn())})
	}
	if identity.GetVersion() != "" {
		fields = append(fields, generatedField{name: "version", value: sourceStringLiteral(identity.GetVersion())})
	}
	if len(identity.GetAliases()) > 0 {
		values := make([]string, 0, len(identity.GetAliases()))
		for _, alias := range identity.GetAliases() {
			values = append(values, sourceStringLiteral(alias))
		}
		fields = append(fields, generatedField{name: "aliases", value: rubyArrayExpr(indent+1, values)})
	}
	return rubyCallExpr(indent, "::Holons::V1::HolonManifest::Identity.new", fields)
}

func rubySkillExpr(skill *holonsv1.HolonManifest_Skill, indent int) string {
	if skill == nil {
		return "::Holons::V1::HolonManifest::Skill.new"
	}
	fields := make([]generatedField, 0, 4)
	if skill.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(skill.GetName())})
	}
	if skill.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(skill.GetDescription())})
	}
	if skill.GetWhen() != "" {
		fields = append(fields, generatedField{name: "when", value: sourceStringLiteral(skill.GetWhen())})
	}
	if len(skill.GetSteps()) > 0 {
		values := make([]string, 0, len(skill.GetSteps()))
		for _, step := range skill.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "steps", value: rubyArrayExpr(indent+1, values)})
	}
	return rubyCallExpr(indent, "::Holons::V1::HolonManifest::Skill.new", fields)
}

func rubySequenceExpr(sequence *holonsv1.HolonManifest_Sequence, indent int) string {
	if sequence == nil {
		return "::Holons::V1::HolonManifest::Sequence.new"
	}
	fields := make([]generatedField, 0, 4)
	if sequence.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(sequence.GetName())})
	}
	if sequence.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(sequence.GetDescription())})
	}
	if len(sequence.GetParams()) > 0 {
		values := make([]string, 0, len(sequence.GetParams()))
		for _, param := range sequence.GetParams() {
			values = append(values, rubySequenceParamExpr(param, indent+2))
		}
		fields = append(fields, generatedField{name: "params", value: rubyArrayExpr(indent+1, values)})
	}
	if len(sequence.GetSteps()) > 0 {
		values := make([]string, 0, len(sequence.GetSteps()))
		for _, step := range sequence.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "steps", value: rubyArrayExpr(indent+1, values)})
	}
	return rubyCallExpr(indent, "::Holons::V1::HolonManifest::Sequence.new", fields)
}

func rubySequenceParamExpr(param *holonsv1.HolonManifest_Sequence_Param, indent int) string {
	if param == nil {
		return "::Holons::V1::HolonManifest::Sequence::Param.new"
	}
	fields := make([]generatedField, 0, 4)
	if param.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(param.GetName())})
	}
	if param.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(param.GetDescription())})
	}
	if param.GetRequired() {
		fields = append(fields, generatedField{name: "required", value: "true"})
	}
	if param.GetDefault() != "" {
		fields = append(fields, generatedField{name: "default", value: sourceStringLiteral(param.GetDefault())})
	}
	return rubyCallExpr(indent, "::Holons::V1::HolonManifest::Sequence::Param.new", fields)
}

func rubyBuildExpr(build *holonsv1.HolonManifest_Build, indent int) string {
	if build == nil {
		return "::Holons::V1::HolonManifest::Build.new"
	}
	fields := make([]generatedField, 0, 3)
	if build.GetRunner() != "" {
		fields = append(fields, generatedField{name: "runner", value: sourceStringLiteral(build.GetRunner())})
	}
	if build.GetMain() != "" {
		fields = append(fields, generatedField{name: "main", value: sourceStringLiteral(build.GetMain())})
	}
	if len(build.GetTemplates()) > 0 {
		values := make([]string, 0, len(build.GetTemplates()))
		for _, template := range build.GetTemplates() {
			values = append(values, sourceStringLiteral(template))
		}
		fields = append(fields, generatedField{name: "templates", value: rubyArrayExpr(indent+1, values)})
	}
	return rubyCallExpr(indent, "::Holons::V1::HolonManifest::Build.new", fields)
}

func rubyRequiresExpr(requires *holonsv1.HolonManifest_Requires, indent int) string {
	if requires == nil {
		return "::Holons::V1::HolonManifest::Requires.new"
	}
	fields := make([]generatedField, 0, 3)
	if len(requires.GetCommands()) > 0 {
		values := make([]string, 0, len(requires.GetCommands()))
		for _, command := range requires.GetCommands() {
			values = append(values, sourceStringLiteral(command))
		}
		fields = append(fields, generatedField{name: "commands", value: rubyArrayExpr(indent+1, values)})
	}
	if len(requires.GetFiles()) > 0 {
		values := make([]string, 0, len(requires.GetFiles()))
		for _, file := range requires.GetFiles() {
			values = append(values, sourceStringLiteral(file))
		}
		fields = append(fields, generatedField{name: "files", value: rubyArrayExpr(indent+1, values)})
	}
	if len(requires.GetPlatforms()) > 0 {
		values := make([]string, 0, len(requires.GetPlatforms()))
		for _, platform := range requires.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "platforms", value: rubyArrayExpr(indent+1, values)})
	}
	return rubyCallExpr(indent, "::Holons::V1::HolonManifest::Requires.new", fields)
}

func rubyArtifactsExpr(artifacts *holonsv1.HolonManifest_Artifacts, indent int) string {
	if artifacts == nil {
		return "::Holons::V1::HolonManifest::Artifacts.new"
	}
	fields := make([]generatedField, 0, 2)
	if artifacts.GetBinary() != "" {
		fields = append(fields, generatedField{name: "binary", value: sourceStringLiteral(artifacts.GetBinary())})
	}
	if artifacts.GetPrimary() != "" {
		fields = append(fields, generatedField{name: "primary", value: sourceStringLiteral(artifacts.GetPrimary())})
	}
	return rubyCallExpr(indent, "::Holons::V1::HolonManifest::Artifacts.new", fields)
}

func rubyServiceDocExpr(service *holonsv1.ServiceDoc, indent int) string {
	if service == nil {
		return "::Holons::V1::ServiceDoc.new"
	}
	fields := make([]generatedField, 0, 3)
	if service.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(service.GetName())})
	}
	if service.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(service.GetDescription())})
	}
	if len(service.GetMethods()) > 0 {
		values := make([]string, 0, len(service.GetMethods()))
		for _, method := range service.GetMethods() {
			values = append(values, rubyMethodDocExpr(method, indent+2))
		}
		fields = append(fields, generatedField{name: "methods", value: rubyArrayExpr(indent+1, values)})
	}
	return rubyCallExpr(indent, "::Holons::V1::ServiceDoc.new", fields)
}

func rubyMethodDocExpr(method *holonsv1.MethodDoc, indent int) string {
	if method == nil {
		return "::Holons::V1::MethodDoc.new"
	}
	fields := make([]generatedField, 0, 9)
	if method.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(method.GetName())})
	}
	if method.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(method.GetDescription())})
	}
	if method.GetInputType() != "" {
		fields = append(fields, generatedField{name: "input_type", value: sourceStringLiteral(method.GetInputType())})
	}
	if method.GetOutputType() != "" {
		fields = append(fields, generatedField{name: "output_type", value: sourceStringLiteral(method.GetOutputType())})
	}
	if len(method.GetInputFields()) > 0 {
		values := make([]string, 0, len(method.GetInputFields()))
		for _, field := range method.GetInputFields() {
			values = append(values, rubyFieldDocExpr(field, indent+2))
		}
		fields = append(fields, generatedField{name: "input_fields", value: rubyArrayExpr(indent+1, values)})
	}
	if len(method.GetOutputFields()) > 0 {
		values := make([]string, 0, len(method.GetOutputFields()))
		for _, field := range method.GetOutputFields() {
			values = append(values, rubyFieldDocExpr(field, indent+2))
		}
		fields = append(fields, generatedField{name: "output_fields", value: rubyArrayExpr(indent+1, values)})
	}
	if method.GetClientStreaming() {
		fields = append(fields, generatedField{name: "client_streaming", value: "true"})
	}
	if method.GetServerStreaming() {
		fields = append(fields, generatedField{name: "server_streaming", value: "true"})
	}
	if method.GetExampleInput() != "" {
		fields = append(fields, generatedField{name: "example_input", value: sourceStringLiteral(method.GetExampleInput())})
	}
	return rubyCallExpr(indent, "::Holons::V1::MethodDoc.new", fields)
}

func rubyFieldDocExpr(field *holonsv1.FieldDoc, indent int) string {
	if field == nil {
		return "::Holons::V1::FieldDoc.new"
	}
	fields := make([]generatedField, 0, 11)
	if field.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(field.GetName())})
	}
	if field.GetType() != "" {
		fields = append(fields, generatedField{name: "type", value: sourceStringLiteral(field.GetType())})
	}
	if field.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "number", value: fmt.Sprintf("%d", field.GetNumber())})
	}
	if field.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(field.GetDescription())})
	}
	fields = append(fields, generatedField{name: "label", value: rubyFieldLabelLiteral(field.GetLabel())})
	if field.GetMapKeyType() != "" {
		fields = append(fields, generatedField{name: "map_key_type", value: sourceStringLiteral(field.GetMapKeyType())})
	}
	if field.GetMapValueType() != "" {
		fields = append(fields, generatedField{name: "map_value_type", value: sourceStringLiteral(field.GetMapValueType())})
	}
	if len(field.GetNestedFields()) > 0 {
		values := make([]string, 0, len(field.GetNestedFields()))
		for _, nested := range field.GetNestedFields() {
			values = append(values, rubyFieldDocExpr(nested, indent+2))
		}
		fields = append(fields, generatedField{name: "nested_fields", value: rubyArrayExpr(indent+1, values)})
	}
	if len(field.GetEnumValues()) > 0 {
		values := make([]string, 0, len(field.GetEnumValues()))
		for _, value := range field.GetEnumValues() {
			values = append(values, rubyEnumValueDocExpr(value, indent+2))
		}
		fields = append(fields, generatedField{name: "enum_values", value: rubyArrayExpr(indent+1, values)})
	}
	if field.GetRequired() {
		fields = append(fields, generatedField{name: "required", value: "true"})
	}
	if field.GetExample() != "" {
		fields = append(fields, generatedField{name: "example", value: sourceStringLiteral(field.GetExample())})
	}
	return rubyCallExpr(indent, "::Holons::V1::FieldDoc.new", fields)
}

func rubyEnumValueDocExpr(value *holonsv1.EnumValueDoc, indent int) string {
	if value == nil {
		return "::Holons::V1::EnumValueDoc.new"
	}
	fields := make([]generatedField, 0, 3)
	if value.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(value.GetName())})
	}
	if value.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "number", value: fmt.Sprintf("%d", value.GetNumber())})
	}
	if value.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(value.GetDescription())})
	}
	return rubyCallExpr(indent, "::Holons::V1::EnumValueDoc.new", fields)
}

func rubyCallExpr(indent int, name string, fields []generatedField) string {
	if len(fields) == 0 {
		return name
	}
	var buf strings.Builder
	buf.WriteString(name)
	buf.WriteString("(\n")
	for _, field := range fields {
		buf.WriteString(genericIndent("  ", indent+1))
		buf.WriteString(field.name)
		buf.WriteString(": ")
		buf.WriteString(field.value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("  ", indent))
	buf.WriteString(")")
	return buf.String()
}

func rubyArrayExpr(indent int, values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for _, value := range values {
		buf.WriteString(genericIndent("  ", indent+1))
		buf.WriteString(value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("  ", indent))
	buf.WriteString("]")
	return buf.String()
}

func rubyFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_UNSPECIFIED:
		return "::Holons::V1::FieldLabel::FIELD_LABEL_UNSPECIFIED"
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return "::Holons::V1::FieldLabel::FIELD_LABEL_OPTIONAL"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return "::Holons::V1::FieldLabel::FIELD_LABEL_REPEATED"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return "::Holons::V1::FieldLabel::FIELD_LABEL_MAP"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return "::Holons::V1::FieldLabel::FIELD_LABEL_REQUIRED"
	default:
		return fmt.Sprintf("%d", value)
	}
}

func jsDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "describe.holons.DescribeResponse.fromObject({})"
	}
	return "describe.holons.DescribeResponse.fromObject(\n" + jsDescribeResponseObject(response, 1) + "\n)"
}

func jsDescribeResponseObject(response *holonsv1.DescribeResponse, indent int) string {
	fields := make([]generatedField, 0, 2)
	if response.GetManifest() != nil {
		fields = append(fields, generatedField{name: "manifest", value: jsHolonManifestObject(response.GetManifest(), indent+1)})
	}
	if len(response.GetServices()) > 0 {
		values := make([]string, 0, len(response.GetServices()))
		for _, service := range response.GetServices() {
			values = append(values, jsServiceDocObject(service, indent+2))
		}
		fields = append(fields, generatedField{name: "services", value: jsArrayExpr(indent+1, values)})
	}
	return jsObjectExpr(indent, fields)
}

func jsHolonManifestObject(manifest *holonsv1.HolonManifest, indent int) string {
	if manifest == nil {
		return "{}"
	}
	fields := make([]generatedField, 0, 11)
	if manifest.GetIdentity() != nil {
		fields = append(fields, generatedField{name: "identity", value: jsIdentityObject(manifest.GetIdentity(), indent+1)})
	}
	if manifest.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(manifest.GetDescription())})
	}
	if manifest.GetLang() != "" {
		fields = append(fields, generatedField{name: "lang", value: sourceStringLiteral(manifest.GetLang())})
	}
	if len(manifest.GetSkills()) > 0 {
		values := make([]string, 0, len(manifest.GetSkills()))
		for _, skill := range manifest.GetSkills() {
			values = append(values, jsSkillObject(skill, indent+2))
		}
		fields = append(fields, generatedField{name: "skills", value: jsArrayExpr(indent+1, values)})
	}
	if manifest.GetKind() != "" {
		fields = append(fields, generatedField{name: "kind", value: sourceStringLiteral(manifest.GetKind())})
	}
	if len(manifest.GetPlatforms()) > 0 {
		values := make([]string, 0, len(manifest.GetPlatforms()))
		for _, platform := range manifest.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "platforms", value: jsArrayExpr(indent+1, values)})
	}
	if manifest.GetTransport() != "" {
		fields = append(fields, generatedField{name: "transport", value: sourceStringLiteral(manifest.GetTransport())})
	}
	if manifest.GetBuild() != nil {
		fields = append(fields, generatedField{name: "build", value: jsBuildObject(manifest.GetBuild(), indent+1)})
	}
	if manifest.GetRequires() != nil {
		fields = append(fields, generatedField{name: "requires", value: jsRequiresObject(manifest.GetRequires(), indent+1)})
	}
	if manifest.GetArtifacts() != nil {
		fields = append(fields, generatedField{name: "artifacts", value: jsArtifactsObject(manifest.GetArtifacts(), indent+1)})
	}
	if len(manifest.GetSequences()) > 0 {
		values := make([]string, 0, len(manifest.GetSequences()))
		for _, sequence := range manifest.GetSequences() {
			values = append(values, jsSequenceObject(sequence, indent+2))
		}
		fields = append(fields, generatedField{name: "sequences", value: jsArrayExpr(indent+1, values)})
	}
	if manifest.GetGuide() != "" {
		fields = append(fields, generatedField{name: "guide", value: sourceStringLiteral(manifest.GetGuide())})
	}
	return jsObjectExpr(indent, fields)
}

func jsIdentityObject(identity *holonsv1.HolonManifest_Identity, indent int) string {
	fields := make([]generatedField, 0, 10)
	if identity.GetSchema() != "" {
		fields = append(fields, generatedField{name: "schema", value: sourceStringLiteral(identity.GetSchema())})
	}
	if identity.GetUuid() != "" {
		fields = append(fields, generatedField{name: "uuid", value: sourceStringLiteral(identity.GetUuid())})
	}
	if identity.GetGivenName() != "" {
		fields = append(fields, generatedField{name: "given_name", value: sourceStringLiteral(identity.GetGivenName())})
	}
	if identity.GetFamilyName() != "" {
		fields = append(fields, generatedField{name: "family_name", value: sourceStringLiteral(identity.GetFamilyName())})
	}
	if identity.GetMotto() != "" {
		fields = append(fields, generatedField{name: "motto", value: sourceStringLiteral(identity.GetMotto())})
	}
	if identity.GetComposer() != "" {
		fields = append(fields, generatedField{name: "composer", value: sourceStringLiteral(identity.GetComposer())})
	}
	if identity.GetStatus() != "" {
		fields = append(fields, generatedField{name: "status", value: sourceStringLiteral(identity.GetStatus())})
	}
	if identity.GetBorn() != "" {
		fields = append(fields, generatedField{name: "born", value: sourceStringLiteral(identity.GetBorn())})
	}
	if identity.GetVersion() != "" {
		fields = append(fields, generatedField{name: "version", value: sourceStringLiteral(identity.GetVersion())})
	}
	if len(identity.GetAliases()) > 0 {
		values := make([]string, 0, len(identity.GetAliases()))
		for _, alias := range identity.GetAliases() {
			values = append(values, sourceStringLiteral(alias))
		}
		fields = append(fields, generatedField{name: "aliases", value: jsArrayExpr(indent+1, values)})
	}
	return jsObjectExpr(indent, fields)
}

func jsSkillObject(skill *holonsv1.HolonManifest_Skill, indent int) string {
	fields := make([]generatedField, 0, 4)
	if skill.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(skill.GetName())})
	}
	if skill.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(skill.GetDescription())})
	}
	if skill.GetWhen() != "" {
		fields = append(fields, generatedField{name: "when", value: sourceStringLiteral(skill.GetWhen())})
	}
	if len(skill.GetSteps()) > 0 {
		values := make([]string, 0, len(skill.GetSteps()))
		for _, step := range skill.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "steps", value: jsArrayExpr(indent+1, values)})
	}
	return jsObjectExpr(indent, fields)
}

func jsSequenceObject(sequence *holonsv1.HolonManifest_Sequence, indent int) string {
	fields := make([]generatedField, 0, 4)
	if sequence.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(sequence.GetName())})
	}
	if sequence.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(sequence.GetDescription())})
	}
	if len(sequence.GetParams()) > 0 {
		values := make([]string, 0, len(sequence.GetParams()))
		for _, param := range sequence.GetParams() {
			values = append(values, jsSequenceParamObject(param, indent+2))
		}
		fields = append(fields, generatedField{name: "params", value: jsArrayExpr(indent+1, values)})
	}
	if len(sequence.GetSteps()) > 0 {
		values := make([]string, 0, len(sequence.GetSteps()))
		for _, step := range sequence.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "steps", value: jsArrayExpr(indent+1, values)})
	}
	return jsObjectExpr(indent, fields)
}

func jsSequenceParamObject(param *holonsv1.HolonManifest_Sequence_Param, indent int) string {
	fields := make([]generatedField, 0, 4)
	if param.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(param.GetName())})
	}
	if param.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(param.GetDescription())})
	}
	if param.GetRequired() {
		fields = append(fields, generatedField{name: "required", value: "true"})
	}
	if param.GetDefault() != "" {
		fields = append(fields, generatedField{name: "default", value: sourceStringLiteral(param.GetDefault())})
	}
	return jsObjectExpr(indent, fields)
}

func jsBuildObject(build *holonsv1.HolonManifest_Build, indent int) string {
	fields := make([]generatedField, 0, 3)
	if build.GetRunner() != "" {
		fields = append(fields, generatedField{name: "runner", value: sourceStringLiteral(build.GetRunner())})
	}
	if build.GetMain() != "" {
		fields = append(fields, generatedField{name: "main", value: sourceStringLiteral(build.GetMain())})
	}
	if len(build.GetTemplates()) > 0 {
		values := make([]string, 0, len(build.GetTemplates()))
		for _, template := range build.GetTemplates() {
			values = append(values, sourceStringLiteral(template))
		}
		fields = append(fields, generatedField{name: "templates", value: jsArrayExpr(indent+1, values)})
	}
	return jsObjectExpr(indent, fields)
}

func jsRequiresObject(requires *holonsv1.HolonManifest_Requires, indent int) string {
	fields := make([]generatedField, 0, 3)
	if len(requires.GetCommands()) > 0 {
		values := make([]string, 0, len(requires.GetCommands()))
		for _, command := range requires.GetCommands() {
			values = append(values, sourceStringLiteral(command))
		}
		fields = append(fields, generatedField{name: "commands", value: jsArrayExpr(indent+1, values)})
	}
	if len(requires.GetFiles()) > 0 {
		values := make([]string, 0, len(requires.GetFiles()))
		for _, file := range requires.GetFiles() {
			values = append(values, sourceStringLiteral(file))
		}
		fields = append(fields, generatedField{name: "files", value: jsArrayExpr(indent+1, values)})
	}
	if len(requires.GetPlatforms()) > 0 {
		values := make([]string, 0, len(requires.GetPlatforms()))
		for _, platform := range requires.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "platforms", value: jsArrayExpr(indent+1, values)})
	}
	return jsObjectExpr(indent, fields)
}

func jsArtifactsObject(artifacts *holonsv1.HolonManifest_Artifacts, indent int) string {
	fields := make([]generatedField, 0, 2)
	if artifacts.GetBinary() != "" {
		fields = append(fields, generatedField{name: "binary", value: sourceStringLiteral(artifacts.GetBinary())})
	}
	if artifacts.GetPrimary() != "" {
		fields = append(fields, generatedField{name: "primary", value: sourceStringLiteral(artifacts.GetPrimary())})
	}
	return jsObjectExpr(indent, fields)
}

func jsServiceDocObject(service *holonsv1.ServiceDoc, indent int) string {
	fields := make([]generatedField, 0, 3)
	if service.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(service.GetName())})
	}
	if service.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(service.GetDescription())})
	}
	if len(service.GetMethods()) > 0 {
		values := make([]string, 0, len(service.GetMethods()))
		for _, method := range service.GetMethods() {
			values = append(values, jsMethodDocObject(method, indent+2))
		}
		fields = append(fields, generatedField{name: "methods", value: jsArrayExpr(indent+1, values)})
	}
	return jsObjectExpr(indent, fields)
}

func jsMethodDocObject(method *holonsv1.MethodDoc, indent int) string {
	fields := make([]generatedField, 0, 9)
	if method.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(method.GetName())})
	}
	if method.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(method.GetDescription())})
	}
	if method.GetInputType() != "" {
		fields = append(fields, generatedField{name: "input_type", value: sourceStringLiteral(method.GetInputType())})
	}
	if method.GetOutputType() != "" {
		fields = append(fields, generatedField{name: "output_type", value: sourceStringLiteral(method.GetOutputType())})
	}
	if len(method.GetInputFields()) > 0 {
		values := make([]string, 0, len(method.GetInputFields()))
		for _, field := range method.GetInputFields() {
			values = append(values, jsFieldDocObject(field, indent+2))
		}
		fields = append(fields, generatedField{name: "input_fields", value: jsArrayExpr(indent+1, values)})
	}
	if len(method.GetOutputFields()) > 0 {
		values := make([]string, 0, len(method.GetOutputFields()))
		for _, field := range method.GetOutputFields() {
			values = append(values, jsFieldDocObject(field, indent+2))
		}
		fields = append(fields, generatedField{name: "output_fields", value: jsArrayExpr(indent+1, values)})
	}
	if method.GetClientStreaming() {
		fields = append(fields, generatedField{name: "client_streaming", value: "true"})
	}
	if method.GetServerStreaming() {
		fields = append(fields, generatedField{name: "server_streaming", value: "true"})
	}
	if method.GetExampleInput() != "" {
		fields = append(fields, generatedField{name: "example_input", value: sourceStringLiteral(method.GetExampleInput())})
	}
	return jsObjectExpr(indent, fields)
}

func jsFieldDocObject(field *holonsv1.FieldDoc, indent int) string {
	fields := make([]generatedField, 0, 11)
	if field.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(field.GetName())})
	}
	if field.GetType() != "" {
		fields = append(fields, generatedField{name: "type", value: sourceStringLiteral(field.GetType())})
	}
	if field.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "number", value: fmt.Sprintf("%d", field.GetNumber())})
	}
	if field.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(field.GetDescription())})
	}
	fields = append(fields, generatedField{name: "label", value: jsFieldLabelLiteral(field.GetLabel())})
	if field.GetMapKeyType() != "" {
		fields = append(fields, generatedField{name: "map_key_type", value: sourceStringLiteral(field.GetMapKeyType())})
	}
	if field.GetMapValueType() != "" {
		fields = append(fields, generatedField{name: "map_value_type", value: sourceStringLiteral(field.GetMapValueType())})
	}
	if len(field.GetNestedFields()) > 0 {
		values := make([]string, 0, len(field.GetNestedFields()))
		for _, nested := range field.GetNestedFields() {
			values = append(values, jsFieldDocObject(nested, indent+2))
		}
		fields = append(fields, generatedField{name: "nested_fields", value: jsArrayExpr(indent+1, values)})
	}
	if len(field.GetEnumValues()) > 0 {
		values := make([]string, 0, len(field.GetEnumValues()))
		for _, value := range field.GetEnumValues() {
			values = append(values, jsEnumValueDocObject(value, indent+2))
		}
		fields = append(fields, generatedField{name: "enum_values", value: jsArrayExpr(indent+1, values)})
	}
	if field.GetRequired() {
		fields = append(fields, generatedField{name: "required", value: "true"})
	}
	if field.GetExample() != "" {
		fields = append(fields, generatedField{name: "example", value: sourceStringLiteral(field.GetExample())})
	}
	return jsObjectExpr(indent, fields)
}

func jsEnumValueDocObject(value *holonsv1.EnumValueDoc, indent int) string {
	fields := make([]generatedField, 0, 3)
	if value.GetName() != "" {
		fields = append(fields, generatedField{name: "name", value: sourceStringLiteral(value.GetName())})
	}
	if value.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "number", value: fmt.Sprintf("%d", value.GetNumber())})
	}
	if value.GetDescription() != "" {
		fields = append(fields, generatedField{name: "description", value: sourceStringLiteral(value.GetDescription())})
	}
	return jsObjectExpr(indent, fields)
}

func jsObjectExpr(indent int, fields []generatedField) string {
	if len(fields) == 0 {
		return "{}"
	}
	var buf strings.Builder
	buf.WriteString("{\n")
	for _, field := range fields {
		buf.WriteString(genericIndent("    ", indent+1))
		buf.WriteString(field.name)
		buf.WriteString(": ")
		buf.WriteString(field.value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("    ", indent))
	buf.WriteString("}")
	return buf.String()
}

func jsArrayExpr(indent int, values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for _, value := range values {
		buf.WriteString(genericIndent("    ", indent+1))
		buf.WriteString(value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("    ", indent))
	buf.WriteString("]")
	return buf.String()
}

func jsFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_UNSPECIFIED:
		return "describe.holons.FieldLabel.FIELD_LABEL_UNSPECIFIED"
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return "describe.holons.FieldLabel.FIELD_LABEL_OPTIONAL"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return "describe.holons.FieldLabel.FIELD_LABEL_REPEATED"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return "describe.holons.FieldLabel.FIELD_LABEL_MAP"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return "describe.holons.FieldLabel.FIELD_LABEL_REQUIRED"
	default:
		return fmt.Sprintf("%d", value)
	}
}

func csharpDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "new DescribeResponse()"
	}
	return csharpDescribeResponseExpr(response, 0)
}

func csharpDescribeResponseExpr(response *holonsv1.DescribeResponse, indent int) string {
	fields := make([]generatedField, 0, 2)
	if response.GetManifest() != nil {
		fields = append(fields, generatedField{name: "Manifest", value: csharpHolonManifestExpr(response.GetManifest(), indent+1)})
	}
	if len(response.GetServices()) > 0 {
		values := make([]string, 0, len(response.GetServices()))
		for _, service := range response.GetServices() {
			values = append(values, csharpServiceDocExpr(service, indent+2))
		}
		fields = append(fields, generatedField{name: "Services", value: csharpCollectionExpr(indent+1, values)})
	}
	return csharpObjectExpr(indent, "DescribeResponse", fields)
}

func csharpHolonManifestExpr(manifest *holonsv1.HolonManifest, indent int) string {
	if manifest == nil {
		return "new HolonManifest()"
	}
	fields := make([]generatedField, 0, 11)
	if manifest.GetIdentity() != nil {
		fields = append(fields, generatedField{name: "Identity", value: csharpIdentityExpr(manifest.GetIdentity(), indent+1)})
	}
	if manifest.GetDescription() != "" {
		fields = append(fields, generatedField{name: "Description", value: sourceStringLiteral(manifest.GetDescription())})
	}
	if manifest.GetLang() != "" {
		fields = append(fields, generatedField{name: "Lang", value: sourceStringLiteral(manifest.GetLang())})
	}
	if len(manifest.GetSkills()) > 0 {
		values := make([]string, 0, len(manifest.GetSkills()))
		for _, skill := range manifest.GetSkills() {
			values = append(values, csharpSkillExpr(skill, indent+2))
		}
		fields = append(fields, generatedField{name: "Skills", value: csharpCollectionExpr(indent+1, values)})
	}
	if manifest.GetKind() != "" {
		fields = append(fields, generatedField{name: "Kind", value: sourceStringLiteral(manifest.GetKind())})
	}
	if len(manifest.GetPlatforms()) > 0 {
		values := make([]string, 0, len(manifest.GetPlatforms()))
		for _, platform := range manifest.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "Platforms", value: csharpCollectionExpr(indent+1, values)})
	}
	if manifest.GetTransport() != "" {
		fields = append(fields, generatedField{name: "Transport", value: sourceStringLiteral(manifest.GetTransport())})
	}
	if manifest.GetBuild() != nil {
		fields = append(fields, generatedField{name: "Build", value: csharpBuildExpr(manifest.GetBuild(), indent+1)})
	}
	if manifest.GetRequires() != nil {
		fields = append(fields, generatedField{name: "Requires", value: csharpRequiresExpr(manifest.GetRequires(), indent+1)})
	}
	if manifest.GetArtifacts() != nil {
		fields = append(fields, generatedField{name: "Artifacts", value: csharpArtifactsExpr(manifest.GetArtifacts(), indent+1)})
	}
	if len(manifest.GetSequences()) > 0 {
		values := make([]string, 0, len(manifest.GetSequences()))
		for _, sequence := range manifest.GetSequences() {
			values = append(values, csharpSequenceExpr(sequence, indent+2))
		}
		fields = append(fields, generatedField{name: "Sequences", value: csharpCollectionExpr(indent+1, values)})
	}
	if manifest.GetGuide() != "" {
		fields = append(fields, generatedField{name: "Guide", value: sourceStringLiteral(manifest.GetGuide())})
	}
	return csharpObjectExpr(indent, "HolonManifest", fields)
}

func csharpIdentityExpr(identity *holonsv1.HolonManifest_Identity, indent int) string {
	fields := make([]generatedField, 0, 10)
	if identity.GetSchema() != "" {
		fields = append(fields, generatedField{name: "Schema", value: sourceStringLiteral(identity.GetSchema())})
	}
	if identity.GetUuid() != "" {
		fields = append(fields, generatedField{name: "Uuid", value: sourceStringLiteral(identity.GetUuid())})
	}
	if identity.GetGivenName() != "" {
		fields = append(fields, generatedField{name: "GivenName", value: sourceStringLiteral(identity.GetGivenName())})
	}
	if identity.GetFamilyName() != "" {
		fields = append(fields, generatedField{name: "FamilyName", value: sourceStringLiteral(identity.GetFamilyName())})
	}
	if identity.GetMotto() != "" {
		fields = append(fields, generatedField{name: "Motto", value: sourceStringLiteral(identity.GetMotto())})
	}
	if identity.GetComposer() != "" {
		fields = append(fields, generatedField{name: "Composer", value: sourceStringLiteral(identity.GetComposer())})
	}
	if identity.GetStatus() != "" {
		fields = append(fields, generatedField{name: "Status", value: sourceStringLiteral(identity.GetStatus())})
	}
	if identity.GetBorn() != "" {
		fields = append(fields, generatedField{name: "Born", value: sourceStringLiteral(identity.GetBorn())})
	}
	if identity.GetVersion() != "" {
		fields = append(fields, generatedField{name: "Version", value: sourceStringLiteral(identity.GetVersion())})
	}
	if len(identity.GetAliases()) > 0 {
		values := make([]string, 0, len(identity.GetAliases()))
		for _, alias := range identity.GetAliases() {
			values = append(values, sourceStringLiteral(alias))
		}
		fields = append(fields, generatedField{name: "Aliases", value: csharpCollectionExpr(indent+1, values)})
	}
	return csharpObjectExpr(indent, "HolonManifest.Types.Identity", fields)
}

func csharpSkillExpr(skill *holonsv1.HolonManifest_Skill, indent int) string {
	fields := make([]generatedField, 0, 4)
	if skill.GetName() != "" {
		fields = append(fields, generatedField{name: "Name", value: sourceStringLiteral(skill.GetName())})
	}
	if skill.GetDescription() != "" {
		fields = append(fields, generatedField{name: "Description", value: sourceStringLiteral(skill.GetDescription())})
	}
	if skill.GetWhen() != "" {
		fields = append(fields, generatedField{name: "When", value: sourceStringLiteral(skill.GetWhen())})
	}
	if len(skill.GetSteps()) > 0 {
		values := make([]string, 0, len(skill.GetSteps()))
		for _, step := range skill.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "Steps", value: csharpCollectionExpr(indent+1, values)})
	}
	return csharpObjectExpr(indent, "HolonManifest.Types.Skill", fields)
}

func csharpSequenceExpr(sequence *holonsv1.HolonManifest_Sequence, indent int) string {
	fields := make([]generatedField, 0, 4)
	if sequence.GetName() != "" {
		fields = append(fields, generatedField{name: "Name", value: sourceStringLiteral(sequence.GetName())})
	}
	if sequence.GetDescription() != "" {
		fields = append(fields, generatedField{name: "Description", value: sourceStringLiteral(sequence.GetDescription())})
	}
	if len(sequence.GetParams()) > 0 {
		values := make([]string, 0, len(sequence.GetParams()))
		for _, param := range sequence.GetParams() {
			values = append(values, csharpSequenceParamExpr(param, indent+2))
		}
		fields = append(fields, generatedField{name: "Params", value: csharpCollectionExpr(indent+1, values)})
	}
	if len(sequence.GetSteps()) > 0 {
		values := make([]string, 0, len(sequence.GetSteps()))
		for _, step := range sequence.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		fields = append(fields, generatedField{name: "Steps", value: csharpCollectionExpr(indent+1, values)})
	}
	return csharpObjectExpr(indent, "HolonManifest.Types.Sequence", fields)
}

func csharpSequenceParamExpr(param *holonsv1.HolonManifest_Sequence_Param, indent int) string {
	fields := make([]generatedField, 0, 4)
	if param.GetName() != "" {
		fields = append(fields, generatedField{name: "Name", value: sourceStringLiteral(param.GetName())})
	}
	if param.GetDescription() != "" {
		fields = append(fields, generatedField{name: "Description", value: sourceStringLiteral(param.GetDescription())})
	}
	if param.GetRequired() {
		fields = append(fields, generatedField{name: "Required", value: "true"})
	}
	if param.GetDefault() != "" {
		fields = append(fields, generatedField{name: "Default", value: sourceStringLiteral(param.GetDefault())})
	}
	return csharpObjectExpr(indent, "HolonManifest.Types.Sequence.Types.Param", fields)
}

func csharpBuildExpr(build *holonsv1.HolonManifest_Build, indent int) string {
	fields := make([]generatedField, 0, 3)
	if build.GetRunner() != "" {
		fields = append(fields, generatedField{name: "Runner", value: sourceStringLiteral(build.GetRunner())})
	}
	if build.GetMain() != "" {
		fields = append(fields, generatedField{name: "Main", value: sourceStringLiteral(build.GetMain())})
	}
	if len(build.GetTemplates()) > 0 {
		values := make([]string, 0, len(build.GetTemplates()))
		for _, template := range build.GetTemplates() {
			values = append(values, sourceStringLiteral(template))
		}
		fields = append(fields, generatedField{name: "Templates", value: csharpCollectionExpr(indent+1, values)})
	}
	return csharpObjectExpr(indent, "HolonManifest.Types.Build", fields)
}

func csharpRequiresExpr(requires *holonsv1.HolonManifest_Requires, indent int) string {
	fields := make([]generatedField, 0, 3)
	if len(requires.GetCommands()) > 0 {
		values := make([]string, 0, len(requires.GetCommands()))
		for _, command := range requires.GetCommands() {
			values = append(values, sourceStringLiteral(command))
		}
		fields = append(fields, generatedField{name: "Commands", value: csharpCollectionExpr(indent+1, values)})
	}
	if len(requires.GetFiles()) > 0 {
		values := make([]string, 0, len(requires.GetFiles()))
		for _, file := range requires.GetFiles() {
			values = append(values, sourceStringLiteral(file))
		}
		fields = append(fields, generatedField{name: "Files", value: csharpCollectionExpr(indent+1, values)})
	}
	if len(requires.GetPlatforms()) > 0 {
		values := make([]string, 0, len(requires.GetPlatforms()))
		for _, platform := range requires.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		fields = append(fields, generatedField{name: "Platforms", value: csharpCollectionExpr(indent+1, values)})
	}
	return csharpObjectExpr(indent, "HolonManifest.Types.Requires", fields)
}

func csharpArtifactsExpr(artifacts *holonsv1.HolonManifest_Artifacts, indent int) string {
	fields := make([]generatedField, 0, 2)
	if artifacts.GetBinary() != "" {
		fields = append(fields, generatedField{name: "Binary", value: sourceStringLiteral(artifacts.GetBinary())})
	}
	if artifacts.GetPrimary() != "" {
		fields = append(fields, generatedField{name: "Primary", value: sourceStringLiteral(artifacts.GetPrimary())})
	}
	return csharpObjectExpr(indent, "HolonManifest.Types.Artifacts", fields)
}

func csharpServiceDocExpr(service *holonsv1.ServiceDoc, indent int) string {
	fields := make([]generatedField, 0, 3)
	if service.GetName() != "" {
		fields = append(fields, generatedField{name: "Name", value: sourceStringLiteral(service.GetName())})
	}
	if service.GetDescription() != "" {
		fields = append(fields, generatedField{name: "Description", value: sourceStringLiteral(service.GetDescription())})
	}
	if len(service.GetMethods()) > 0 {
		values := make([]string, 0, len(service.GetMethods()))
		for _, method := range service.GetMethods() {
			values = append(values, csharpMethodDocExpr(method, indent+2))
		}
		fields = append(fields, generatedField{name: "Methods", value: csharpCollectionExpr(indent+1, values)})
	}
	return csharpObjectExpr(indent, "ServiceDoc", fields)
}

func csharpMethodDocExpr(method *holonsv1.MethodDoc, indent int) string {
	fields := make([]generatedField, 0, 9)
	if method.GetName() != "" {
		fields = append(fields, generatedField{name: "Name", value: sourceStringLiteral(method.GetName())})
	}
	if method.GetDescription() != "" {
		fields = append(fields, generatedField{name: "Description", value: sourceStringLiteral(method.GetDescription())})
	}
	if method.GetInputType() != "" {
		fields = append(fields, generatedField{name: "InputType", value: sourceStringLiteral(method.GetInputType())})
	}
	if method.GetOutputType() != "" {
		fields = append(fields, generatedField{name: "OutputType", value: sourceStringLiteral(method.GetOutputType())})
	}
	if len(method.GetInputFields()) > 0 {
		values := make([]string, 0, len(method.GetInputFields()))
		for _, field := range method.GetInputFields() {
			values = append(values, csharpFieldDocExpr(field, indent+2))
		}
		fields = append(fields, generatedField{name: "InputFields", value: csharpCollectionExpr(indent+1, values)})
	}
	if len(method.GetOutputFields()) > 0 {
		values := make([]string, 0, len(method.GetOutputFields()))
		for _, field := range method.GetOutputFields() {
			values = append(values, csharpFieldDocExpr(field, indent+2))
		}
		fields = append(fields, generatedField{name: "OutputFields", value: csharpCollectionExpr(indent+1, values)})
	}
	if method.GetClientStreaming() {
		fields = append(fields, generatedField{name: "ClientStreaming", value: "true"})
	}
	if method.GetServerStreaming() {
		fields = append(fields, generatedField{name: "ServerStreaming", value: "true"})
	}
	if method.GetExampleInput() != "" {
		fields = append(fields, generatedField{name: "ExampleInput", value: sourceStringLiteral(method.GetExampleInput())})
	}
	return csharpObjectExpr(indent, "MethodDoc", fields)
}

func csharpFieldDocExpr(field *holonsv1.FieldDoc, indent int) string {
	fields := make([]generatedField, 0, 11)
	if field.GetName() != "" {
		fields = append(fields, generatedField{name: "Name", value: sourceStringLiteral(field.GetName())})
	}
	if field.GetType() != "" {
		fields = append(fields, generatedField{name: "Type", value: sourceStringLiteral(field.GetType())})
	}
	if field.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "Number", value: fmt.Sprintf("%d", field.GetNumber())})
	}
	if field.GetDescription() != "" {
		fields = append(fields, generatedField{name: "Description", value: sourceStringLiteral(field.GetDescription())})
	}
	fields = append(fields, generatedField{name: "Label", value: csharpFieldLabelLiteral(field.GetLabel())})
	if field.GetMapKeyType() != "" {
		fields = append(fields, generatedField{name: "MapKeyType", value: sourceStringLiteral(field.GetMapKeyType())})
	}
	if field.GetMapValueType() != "" {
		fields = append(fields, generatedField{name: "MapValueType", value: sourceStringLiteral(field.GetMapValueType())})
	}
	if len(field.GetNestedFields()) > 0 {
		values := make([]string, 0, len(field.GetNestedFields()))
		for _, nested := range field.GetNestedFields() {
			values = append(values, csharpFieldDocExpr(nested, indent+2))
		}
		fields = append(fields, generatedField{name: "NestedFields", value: csharpCollectionExpr(indent+1, values)})
	}
	if len(field.GetEnumValues()) > 0 {
		values := make([]string, 0, len(field.GetEnumValues()))
		for _, value := range field.GetEnumValues() {
			values = append(values, csharpEnumValueDocExpr(value, indent+2))
		}
		fields = append(fields, generatedField{name: "EnumValues", value: csharpCollectionExpr(indent+1, values)})
	}
	if field.GetRequired() {
		fields = append(fields, generatedField{name: "Required", value: "true"})
	}
	if field.GetExample() != "" {
		fields = append(fields, generatedField{name: "Example", value: sourceStringLiteral(field.GetExample())})
	}
	return csharpObjectExpr(indent, "FieldDoc", fields)
}

func csharpEnumValueDocExpr(value *holonsv1.EnumValueDoc, indent int) string {
	fields := make([]generatedField, 0, 3)
	if value.GetName() != "" {
		fields = append(fields, generatedField{name: "Name", value: sourceStringLiteral(value.GetName())})
	}
	if value.GetNumber() != 0 {
		fields = append(fields, generatedField{name: "Number", value: fmt.Sprintf("%d", value.GetNumber())})
	}
	if value.GetDescription() != "" {
		fields = append(fields, generatedField{name: "Description", value: sourceStringLiteral(value.GetDescription())})
	}
	return csharpObjectExpr(indent, "EnumValueDoc", fields)
}

func csharpObjectExpr(indent int, typeName string, fields []generatedField) string {
	if len(fields) == 0 {
		return "new " + typeName + "()"
	}
	var buf strings.Builder
	buf.WriteString("new ")
	buf.WriteString(typeName)
	buf.WriteString("\n")
	buf.WriteString(genericIndent("    ", indent))
	buf.WriteString("{\n")
	for _, field := range fields {
		buf.WriteString(genericIndent("    ", indent+1))
		buf.WriteString(field.name)
		buf.WriteString(" = ")
		buf.WriteString(field.value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("    ", indent))
	buf.WriteString("}")
	return buf.String()
}

func csharpCollectionExpr(indent int, values []string) string {
	if len(values) == 0 {
		return "{ }"
	}
	var buf strings.Builder
	buf.WriteString("{\n")
	for _, value := range values {
		buf.WriteString(genericIndent("    ", indent+1))
		buf.WriteString(value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("    ", indent))
	buf.WriteString("}")
	return buf.String()
}

func csharpFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_UNSPECIFIED:
		return "FieldLabel.Unspecified"
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return "FieldLabel.Optional"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return "FieldLabel.Repeated"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return "FieldLabel.Map"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return "FieldLabel.Required"
	default:
		return fmt.Sprintf("(FieldLabel)%d", value)
	}
}

func javaDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "holons.v1.Describe.DescribeResponse.newBuilder().build()"
	}
	return javaDescribeResponseExpr(response, 0)
}

func javaDescribeResponseExpr(response *holonsv1.DescribeResponse, indent int) string {
	var buf strings.Builder
	writeJavaLine(&buf, indent, "holons.v1.Describe.DescribeResponse.newBuilder()")
	if response.GetManifest() != nil {
		writeJavaLine(&buf, indent+1, ".setManifest(")
		writeJavaDescribeManifestExpr(&buf, response.GetManifest(), indent+2)
		writeJavaLine(&buf, indent+1, ")")
	}
	for _, service := range response.GetServices() {
		writeJavaLine(&buf, indent+1, ".addServices(")
		writeJavaServiceDocExpr(&buf, service, indent+2)
		writeJavaLine(&buf, indent+1, ")")
	}
	writeJavaLine(&buf, indent+1, ".build()")
	return strings.TrimSuffix(buf.String(), "\n")
}

func writeJavaDescribeManifestExpr(buf *strings.Builder, manifest *holonsv1.HolonManifest, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Manifest.HolonManifest.newBuilder()")
	if manifest.GetIdentity() != nil {
		writeJavaLine(buf, indent+1, ".setIdentity(")
		writeJavaIdentityExpr(buf, manifest.GetIdentity(), indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	writeJavaStringSetter(buf, indent+1, "Description", manifest.GetDescription())
	writeJavaStringSetter(buf, indent+1, "Lang", manifest.GetLang())
	for _, skill := range manifest.GetSkills() {
		writeJavaLine(buf, indent+1, ".addSkills(")
		writeJavaSkillExpr(buf, skill, indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	writeJavaStringSetter(buf, indent+1, "Kind", manifest.GetKind())
	for _, platform := range manifest.GetPlatforms() {
		writeJavaLine(buf, indent+1, fmt.Sprintf(".addPlatforms(%s)", sourceStringLiteral(platform)))
	}
	writeJavaStringSetter(buf, indent+1, "Transport", manifest.GetTransport())
	if manifest.GetBuild() != nil {
		writeJavaLine(buf, indent+1, ".setBuild(")
		writeJavaBuildExpr(buf, manifest.GetBuild(), indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	if manifest.GetRequires() != nil {
		writeJavaLine(buf, indent+1, ".setRequires(")
		writeJavaRequiresExpr(buf, manifest.GetRequires(), indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	if manifest.GetArtifacts() != nil {
		writeJavaLine(buf, indent+1, ".setArtifacts(")
		writeJavaArtifactsExpr(buf, manifest.GetArtifacts(), indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	for _, sequence := range manifest.GetSequences() {
		writeJavaLine(buf, indent+1, ".addSequences(")
		writeJavaSequenceExpr(buf, sequence, indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	writeJavaStringSetter(buf, indent+1, "Guide", manifest.GetGuide())
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaIdentityExpr(buf *strings.Builder, identity *holonsv1.HolonManifest_Identity, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Manifest.HolonManifest.Identity.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Schema", identity.GetSchema())
	writeJavaStringSetter(buf, indent+1, "Uuid", identity.GetUuid())
	writeJavaStringSetter(buf, indent+1, "GivenName", identity.GetGivenName())
	writeJavaStringSetter(buf, indent+1, "FamilyName", identity.GetFamilyName())
	writeJavaStringSetter(buf, indent+1, "Motto", identity.GetMotto())
	writeJavaStringSetter(buf, indent+1, "Composer", identity.GetComposer())
	writeJavaStringSetter(buf, indent+1, "Status", identity.GetStatus())
	writeJavaStringSetter(buf, indent+1, "Born", identity.GetBorn())
	writeJavaStringSetter(buf, indent+1, "Version", identity.GetVersion())
	for _, alias := range identity.GetAliases() {
		writeJavaLine(buf, indent+1, fmt.Sprintf(".addAliases(%s)", sourceStringLiteral(alias)))
	}
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaSkillExpr(buf *strings.Builder, skill *holonsv1.HolonManifest_Skill, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Manifest.HolonManifest.Skill.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Name", skill.GetName())
	writeJavaStringSetter(buf, indent+1, "Description", skill.GetDescription())
	writeJavaStringSetter(buf, indent+1, "When", skill.GetWhen())
	for _, step := range skill.GetSteps() {
		writeJavaLine(buf, indent+1, fmt.Sprintf(".addSteps(%s)", sourceStringLiteral(step)))
	}
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaSequenceExpr(buf *strings.Builder, sequence *holonsv1.HolonManifest_Sequence, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Manifest.HolonManifest.Sequence.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Name", sequence.GetName())
	writeJavaStringSetter(buf, indent+1, "Description", sequence.GetDescription())
	for _, param := range sequence.GetParams() {
		writeJavaLine(buf, indent+1, ".addParams(")
		writeJavaSequenceParamExpr(buf, param, indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	for _, step := range sequence.GetSteps() {
		writeJavaLine(buf, indent+1, fmt.Sprintf(".addSteps(%s)", sourceStringLiteral(step)))
	}
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaSequenceParamExpr(buf *strings.Builder, param *holonsv1.HolonManifest_Sequence_Param, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Manifest.HolonManifest.Sequence.Param.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Name", param.GetName())
	writeJavaStringSetter(buf, indent+1, "Description", param.GetDescription())
	writeJavaBoolSetter(buf, indent+1, "Required", param.GetRequired())
	writeJavaStringSetter(buf, indent+1, "Default", param.GetDefault())
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaBuildExpr(buf *strings.Builder, build *holonsv1.HolonManifest_Build, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Manifest.HolonManifest.Build.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Runner", build.GetRunner())
	writeJavaStringSetter(buf, indent+1, "Main", build.GetMain())
	for _, template := range build.GetTemplates() {
		writeJavaLine(buf, indent+1, fmt.Sprintf(".addTemplates(%s)", sourceStringLiteral(template)))
	}
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaRequiresExpr(buf *strings.Builder, requires *holonsv1.HolonManifest_Requires, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Manifest.HolonManifest.Requires.newBuilder()")
	for _, command := range requires.GetCommands() {
		writeJavaLine(buf, indent+1, fmt.Sprintf(".addCommands(%s)", sourceStringLiteral(command)))
	}
	for _, file := range requires.GetFiles() {
		writeJavaLine(buf, indent+1, fmt.Sprintf(".addFiles(%s)", sourceStringLiteral(file)))
	}
	for _, platform := range requires.GetPlatforms() {
		writeJavaLine(buf, indent+1, fmt.Sprintf(".addPlatforms(%s)", sourceStringLiteral(platform)))
	}
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaArtifactsExpr(buf *strings.Builder, artifacts *holonsv1.HolonManifest_Artifacts, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Manifest.HolonManifest.Artifacts.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Binary", artifacts.GetBinary())
	writeJavaStringSetter(buf, indent+1, "Primary", artifacts.GetPrimary())
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaServiceDocExpr(buf *strings.Builder, service *holonsv1.ServiceDoc, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Describe.ServiceDoc.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Name", service.GetName())
	writeJavaStringSetter(buf, indent+1, "Description", service.GetDescription())
	for _, method := range service.GetMethods() {
		writeJavaLine(buf, indent+1, ".addMethods(")
		writeJavaMethodDocExpr(buf, method, indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaMethodDocExpr(buf *strings.Builder, method *holonsv1.MethodDoc, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Describe.MethodDoc.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Name", method.GetName())
	writeJavaStringSetter(buf, indent+1, "Description", method.GetDescription())
	writeJavaStringSetter(buf, indent+1, "InputType", method.GetInputType())
	writeJavaStringSetter(buf, indent+1, "OutputType", method.GetOutputType())
	for _, field := range method.GetInputFields() {
		writeJavaLine(buf, indent+1, ".addInputFields(")
		writeJavaFieldDocExpr(buf, field, indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	for _, field := range method.GetOutputFields() {
		writeJavaLine(buf, indent+1, ".addOutputFields(")
		writeJavaFieldDocExpr(buf, field, indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	writeJavaBoolSetter(buf, indent+1, "ClientStreaming", method.GetClientStreaming())
	writeJavaBoolSetter(buf, indent+1, "ServerStreaming", method.GetServerStreaming())
	writeJavaStringSetter(buf, indent+1, "ExampleInput", method.GetExampleInput())
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaFieldDocExpr(buf *strings.Builder, field *holonsv1.FieldDoc, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Describe.FieldDoc.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Name", field.GetName())
	writeJavaStringSetter(buf, indent+1, "Type", field.GetType())
	writeJavaInt32Setter(buf, indent+1, "Number", field.GetNumber())
	writeJavaStringSetter(buf, indent+1, "Description", field.GetDescription())
	writeJavaLine(buf, indent+1, fmt.Sprintf(".setLabel(%s)", javaFieldLabelLiteral(field.GetLabel())))
	writeJavaStringSetter(buf, indent+1, "MapKeyType", field.GetMapKeyType())
	writeJavaStringSetter(buf, indent+1, "MapValueType", field.GetMapValueType())
	for _, nested := range field.GetNestedFields() {
		writeJavaLine(buf, indent+1, ".addNestedFields(")
		writeJavaFieldDocExpr(buf, nested, indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	for _, value := range field.GetEnumValues() {
		writeJavaLine(buf, indent+1, ".addEnumValues(")
		writeJavaEnumValueDocExpr(buf, value, indent+2)
		writeJavaLine(buf, indent+1, ")")
	}
	writeJavaBoolSetter(buf, indent+1, "Required", field.GetRequired())
	writeJavaStringSetter(buf, indent+1, "Example", field.GetExample())
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaEnumValueDocExpr(buf *strings.Builder, value *holonsv1.EnumValueDoc, indent int) {
	writeJavaLine(buf, indent, "holons.v1.Describe.EnumValueDoc.newBuilder()")
	writeJavaStringSetter(buf, indent+1, "Name", value.GetName())
	writeJavaInt32Setter(buf, indent+1, "Number", value.GetNumber())
	writeJavaStringSetter(buf, indent+1, "Description", value.GetDescription())
	writeJavaLine(buf, indent+1, ".build()")
}

func writeJavaLine(buf *strings.Builder, indent int, line string) {
	buf.WriteString(genericIndent("    ", indent))
	buf.WriteString(line)
	buf.WriteString("\n")
}

func writeJavaStringSetter(buf *strings.Builder, indent int, fieldName, value string) {
	if value == "" {
		return
	}
	writeJavaLine(buf, indent, fmt.Sprintf(".set%s(%s)", fieldName, sourceStringLiteral(value)))
}

func writeJavaBoolSetter(buf *strings.Builder, indent int, fieldName string, value bool) {
	if !value {
		return
	}
	writeJavaLine(buf, indent, fmt.Sprintf(".set%s(true)", fieldName))
}

func writeJavaInt32Setter(buf *strings.Builder, indent int, fieldName string, value int32) {
	if value == 0 {
		return
	}
	writeJavaLine(buf, indent, fmt.Sprintf(".set%s(%d)", fieldName, value))
}

func javaFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_UNSPECIFIED:
		return "holons.v1.Describe.FieldLabel.FIELD_LABEL_UNSPECIFIED"
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return "holons.v1.Describe.FieldLabel.FIELD_LABEL_OPTIONAL"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return "holons.v1.Describe.FieldLabel.FIELD_LABEL_REPEATED"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return "holons.v1.Describe.FieldLabel.FIELD_LABEL_MAP"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return "holons.v1.Describe.FieldLabel.FIELD_LABEL_REQUIRED"
	default:
		return fmt.Sprintf("holons.v1.Describe.FieldLabel.forNumber(%d)", value)
	}
}

func kotlinDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "holons.v1.Describe.DescribeResponse.newBuilder().build()"
	}
	return javaDescribeResponseExpr(response, 0)
}

func swiftDescribeResponseLiteral(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return "Holons_V1_DescribeResponse()"
	}
	return swiftDescribeResponseExpr(response, 0)
}

func swiftDescribeResponseExpr(response *holonsv1.DescribeResponse, indent int) string {
	assignments := make([]generatedField, 0, 2)
	if response.GetManifest() != nil {
		assignments = append(assignments, generatedField{name: "manifest", value: swiftHolonManifestExpr(response.GetManifest(), indent+1)})
	}
	if len(response.GetServices()) > 0 {
		values := make([]string, 0, len(response.GetServices()))
		for _, service := range response.GetServices() {
			values = append(values, swiftServiceDocExpr(service, indent+2))
		}
		assignments = append(assignments, generatedField{name: "services", value: swiftArrayExpr(indent+1, values)})
	}
	return swiftWithExpr(indent, "Holons_V1_DescribeResponse", assignments)
}

func swiftHolonManifestExpr(manifest *holonsv1.HolonManifest, indent int) string {
	assignments := make([]generatedField, 0, 11)
	if manifest.GetIdentity() != nil {
		assignments = append(assignments, generatedField{name: "identity", value: swiftIdentityExpr(manifest.GetIdentity(), indent+1)})
	}
	if manifest.GetDescription() != "" {
		assignments = append(assignments, generatedField{name: "description_p", value: sourceStringLiteral(manifest.GetDescription())})
	}
	if manifest.GetLang() != "" {
		assignments = append(assignments, generatedField{name: "lang", value: sourceStringLiteral(manifest.GetLang())})
	}
	if len(manifest.GetSkills()) > 0 {
		values := make([]string, 0, len(manifest.GetSkills()))
		for _, skill := range manifest.GetSkills() {
			values = append(values, swiftSkillExpr(skill, indent+2))
		}
		assignments = append(assignments, generatedField{name: "skills", value: swiftArrayExpr(indent+1, values)})
	}
	if manifest.GetKind() != "" {
		assignments = append(assignments, generatedField{name: "kind", value: sourceStringLiteral(manifest.GetKind())})
	}
	if len(manifest.GetPlatforms()) > 0 {
		values := make([]string, 0, len(manifest.GetPlatforms()))
		for _, platform := range manifest.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		assignments = append(assignments, generatedField{name: "platforms", value: swiftArrayExpr(indent+1, values)})
	}
	if manifest.GetTransport() != "" {
		assignments = append(assignments, generatedField{name: "transport", value: sourceStringLiteral(manifest.GetTransport())})
	}
	if manifest.GetBuild() != nil {
		assignments = append(assignments, generatedField{name: "build", value: swiftBuildExpr(manifest.GetBuild(), indent+1)})
	}
	if manifest.GetRequires() != nil {
		assignments = append(assignments, generatedField{name: "requires", value: swiftRequiresExpr(manifest.GetRequires(), indent+1)})
	}
	if manifest.GetArtifacts() != nil {
		assignments = append(assignments, generatedField{name: "artifacts", value: swiftArtifactsExpr(manifest.GetArtifacts(), indent+1)})
	}
	if len(manifest.GetSequences()) > 0 {
		values := make([]string, 0, len(manifest.GetSequences()))
		for _, sequence := range manifest.GetSequences() {
			values = append(values, swiftSequenceExpr(sequence, indent+2))
		}
		assignments = append(assignments, generatedField{name: "sequences", value: swiftArrayExpr(indent+1, values)})
	}
	if manifest.GetGuide() != "" {
		assignments = append(assignments, generatedField{name: "guide", value: sourceStringLiteral(manifest.GetGuide())})
	}
	return swiftWithExpr(indent, "Holons_V1_HolonManifest", assignments)
}

func swiftIdentityExpr(identity *holonsv1.HolonManifest_Identity, indent int) string {
	assignments := make([]generatedField, 0, 10)
	if identity.GetSchema() != "" {
		assignments = append(assignments, generatedField{name: "schema", value: sourceStringLiteral(identity.GetSchema())})
	}
	if identity.GetUuid() != "" {
		assignments = append(assignments, generatedField{name: "uuid", value: sourceStringLiteral(identity.GetUuid())})
	}
	if identity.GetGivenName() != "" {
		assignments = append(assignments, generatedField{name: "givenName", value: sourceStringLiteral(identity.GetGivenName())})
	}
	if identity.GetFamilyName() != "" {
		assignments = append(assignments, generatedField{name: "familyName", value: sourceStringLiteral(identity.GetFamilyName())})
	}
	if identity.GetMotto() != "" {
		assignments = append(assignments, generatedField{name: "motto", value: sourceStringLiteral(identity.GetMotto())})
	}
	if identity.GetComposer() != "" {
		assignments = append(assignments, generatedField{name: "composer", value: sourceStringLiteral(identity.GetComposer())})
	}
	if identity.GetStatus() != "" {
		assignments = append(assignments, generatedField{name: "status", value: sourceStringLiteral(identity.GetStatus())})
	}
	if identity.GetBorn() != "" {
		assignments = append(assignments, generatedField{name: "born", value: sourceStringLiteral(identity.GetBorn())})
	}
	if identity.GetVersion() != "" {
		assignments = append(assignments, generatedField{name: "version", value: sourceStringLiteral(identity.GetVersion())})
	}
	if len(identity.GetAliases()) > 0 {
		values := make([]string, 0, len(identity.GetAliases()))
		for _, alias := range identity.GetAliases() {
			values = append(values, sourceStringLiteral(alias))
		}
		assignments = append(assignments, generatedField{name: "aliases", value: swiftArrayExpr(indent+1, values)})
	}
	return swiftWithExpr(indent, "Holons_V1_HolonManifest.Identity", assignments)
}

func swiftSkillExpr(skill *holonsv1.HolonManifest_Skill, indent int) string {
	assignments := make([]generatedField, 0, 4)
	if skill.GetName() != "" {
		assignments = append(assignments, generatedField{name: "name", value: sourceStringLiteral(skill.GetName())})
	}
	if skill.GetDescription() != "" {
		assignments = append(assignments, generatedField{name: "description_p", value: sourceStringLiteral(skill.GetDescription())})
	}
	if skill.GetWhen() != "" {
		assignments = append(assignments, generatedField{name: "when", value: sourceStringLiteral(skill.GetWhen())})
	}
	if len(skill.GetSteps()) > 0 {
		values := make([]string, 0, len(skill.GetSteps()))
		for _, step := range skill.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		assignments = append(assignments, generatedField{name: "steps", value: swiftArrayExpr(indent+1, values)})
	}
	return swiftWithExpr(indent, "Holons_V1_HolonManifest.Skill", assignments)
}

func swiftSequenceExpr(sequence *holonsv1.HolonManifest_Sequence, indent int) string {
	assignments := make([]generatedField, 0, 4)
	if sequence.GetName() != "" {
		assignments = append(assignments, generatedField{name: "name", value: sourceStringLiteral(sequence.GetName())})
	}
	if sequence.GetDescription() != "" {
		assignments = append(assignments, generatedField{name: "description_p", value: sourceStringLiteral(sequence.GetDescription())})
	}
	if len(sequence.GetParams()) > 0 {
		values := make([]string, 0, len(sequence.GetParams()))
		for _, param := range sequence.GetParams() {
			values = append(values, swiftSequenceParamExpr(param, indent+2))
		}
		assignments = append(assignments, generatedField{name: "params", value: swiftArrayExpr(indent+1, values)})
	}
	if len(sequence.GetSteps()) > 0 {
		values := make([]string, 0, len(sequence.GetSteps()))
		for _, step := range sequence.GetSteps() {
			values = append(values, sourceStringLiteral(step))
		}
		assignments = append(assignments, generatedField{name: "steps", value: swiftArrayExpr(indent+1, values)})
	}
	return swiftWithExpr(indent, "Holons_V1_HolonManifest.Sequence", assignments)
}

func swiftSequenceParamExpr(param *holonsv1.HolonManifest_Sequence_Param, indent int) string {
	assignments := make([]generatedField, 0, 4)
	if param.GetName() != "" {
		assignments = append(assignments, generatedField{name: "name", value: sourceStringLiteral(param.GetName())})
	}
	if param.GetDescription() != "" {
		assignments = append(assignments, generatedField{name: "description_p", value: sourceStringLiteral(param.GetDescription())})
	}
	if param.GetRequired() {
		assignments = append(assignments, generatedField{name: "required", value: "true"})
	}
	if param.GetDefault() != "" {
		assignments = append(assignments, generatedField{name: "default", value: sourceStringLiteral(param.GetDefault())})
	}
	return swiftWithExpr(indent, "Holons_V1_HolonManifest.Sequence.Param", assignments)
}

func swiftBuildExpr(build *holonsv1.HolonManifest_Build, indent int) string {
	assignments := make([]generatedField, 0, 3)
	if build.GetRunner() != "" {
		assignments = append(assignments, generatedField{name: "runner", value: sourceStringLiteral(build.GetRunner())})
	}
	if build.GetMain() != "" {
		assignments = append(assignments, generatedField{name: "main", value: sourceStringLiteral(build.GetMain())})
	}
	if len(build.GetTemplates()) > 0 {
		values := make([]string, 0, len(build.GetTemplates()))
		for _, template := range build.GetTemplates() {
			values = append(values, sourceStringLiteral(template))
		}
		assignments = append(assignments, generatedField{name: "templates", value: swiftArrayExpr(indent+1, values)})
	}
	return swiftWithExpr(indent, "Holons_V1_HolonManifest.Build", assignments)
}

func swiftRequiresExpr(requires *holonsv1.HolonManifest_Requires, indent int) string {
	assignments := make([]generatedField, 0, 3)
	if len(requires.GetCommands()) > 0 {
		values := make([]string, 0, len(requires.GetCommands()))
		for _, command := range requires.GetCommands() {
			values = append(values, sourceStringLiteral(command))
		}
		assignments = append(assignments, generatedField{name: "commands", value: swiftArrayExpr(indent+1, values)})
	}
	if len(requires.GetFiles()) > 0 {
		values := make([]string, 0, len(requires.GetFiles()))
		for _, file := range requires.GetFiles() {
			values = append(values, sourceStringLiteral(file))
		}
		assignments = append(assignments, generatedField{name: "files", value: swiftArrayExpr(indent+1, values)})
	}
	if len(requires.GetPlatforms()) > 0 {
		values := make([]string, 0, len(requires.GetPlatforms()))
		for _, platform := range requires.GetPlatforms() {
			values = append(values, sourceStringLiteral(platform))
		}
		assignments = append(assignments, generatedField{name: "platforms", value: swiftArrayExpr(indent+1, values)})
	}
	return swiftWithExpr(indent, "Holons_V1_HolonManifest.Requires", assignments)
}

func swiftArtifactsExpr(artifacts *holonsv1.HolonManifest_Artifacts, indent int) string {
	assignments := make([]generatedField, 0, 2)
	if artifacts.GetBinary() != "" {
		assignments = append(assignments, generatedField{name: "binary", value: sourceStringLiteral(artifacts.GetBinary())})
	}
	if artifacts.GetPrimary() != "" {
		assignments = append(assignments, generatedField{name: "primary", value: sourceStringLiteral(artifacts.GetPrimary())})
	}
	return swiftWithExpr(indent, "Holons_V1_HolonManifest.Artifacts", assignments)
}

func swiftServiceDocExpr(service *holonsv1.ServiceDoc, indent int) string {
	assignments := make([]generatedField, 0, 3)
	if service.GetName() != "" {
		assignments = append(assignments, generatedField{name: "name", value: sourceStringLiteral(service.GetName())})
	}
	if service.GetDescription() != "" {
		assignments = append(assignments, generatedField{name: "description_p", value: sourceStringLiteral(service.GetDescription())})
	}
	if len(service.GetMethods()) > 0 {
		values := make([]string, 0, len(service.GetMethods()))
		for _, method := range service.GetMethods() {
			values = append(values, swiftMethodDocExpr(method, indent+2))
		}
		assignments = append(assignments, generatedField{name: "methods", value: swiftArrayExpr(indent+1, values)})
	}
	return swiftWithExpr(indent, "Holons_V1_ServiceDoc", assignments)
}

func swiftMethodDocExpr(method *holonsv1.MethodDoc, indent int) string {
	assignments := make([]generatedField, 0, 9)
	if method.GetName() != "" {
		assignments = append(assignments, generatedField{name: "name", value: sourceStringLiteral(method.GetName())})
	}
	if method.GetDescription() != "" {
		assignments = append(assignments, generatedField{name: "description_p", value: sourceStringLiteral(method.GetDescription())})
	}
	if method.GetInputType() != "" {
		assignments = append(assignments, generatedField{name: "inputType", value: sourceStringLiteral(method.GetInputType())})
	}
	if method.GetOutputType() != "" {
		assignments = append(assignments, generatedField{name: "outputType", value: sourceStringLiteral(method.GetOutputType())})
	}
	if len(method.GetInputFields()) > 0 {
		values := make([]string, 0, len(method.GetInputFields()))
		for _, field := range method.GetInputFields() {
			values = append(values, swiftFieldDocExpr(field, indent+2))
		}
		assignments = append(assignments, generatedField{name: "inputFields", value: swiftArrayExpr(indent+1, values)})
	}
	if len(method.GetOutputFields()) > 0 {
		values := make([]string, 0, len(method.GetOutputFields()))
		for _, field := range method.GetOutputFields() {
			values = append(values, swiftFieldDocExpr(field, indent+2))
		}
		assignments = append(assignments, generatedField{name: "outputFields", value: swiftArrayExpr(indent+1, values)})
	}
	if method.GetClientStreaming() {
		assignments = append(assignments, generatedField{name: "clientStreaming", value: "true"})
	}
	if method.GetServerStreaming() {
		assignments = append(assignments, generatedField{name: "serverStreaming", value: "true"})
	}
	if method.GetExampleInput() != "" {
		assignments = append(assignments, generatedField{name: "exampleInput", value: sourceStringLiteral(method.GetExampleInput())})
	}
	return swiftWithExpr(indent, "Holons_V1_MethodDoc", assignments)
}

func swiftFieldDocExpr(field *holonsv1.FieldDoc, indent int) string {
	assignments := make([]generatedField, 0, 11)
	if field.GetName() != "" {
		assignments = append(assignments, generatedField{name: "name", value: sourceStringLiteral(field.GetName())})
	}
	if field.GetType() != "" {
		assignments = append(assignments, generatedField{name: "type", value: sourceStringLiteral(field.GetType())})
	}
	if field.GetNumber() != 0 {
		assignments = append(assignments, generatedField{name: "number", value: fmt.Sprintf("%d", field.GetNumber())})
	}
	if field.GetDescription() != "" {
		assignments = append(assignments, generatedField{name: "description_p", value: sourceStringLiteral(field.GetDescription())})
	}
	assignments = append(assignments, generatedField{name: "label", value: swiftFieldLabelLiteral(field.GetLabel())})
	if field.GetMapKeyType() != "" {
		assignments = append(assignments, generatedField{name: "mapKeyType", value: sourceStringLiteral(field.GetMapKeyType())})
	}
	if field.GetMapValueType() != "" {
		assignments = append(assignments, generatedField{name: "mapValueType", value: sourceStringLiteral(field.GetMapValueType())})
	}
	if len(field.GetNestedFields()) > 0 {
		values := make([]string, 0, len(field.GetNestedFields()))
		for _, nested := range field.GetNestedFields() {
			values = append(values, swiftFieldDocExpr(nested, indent+2))
		}
		assignments = append(assignments, generatedField{name: "nestedFields", value: swiftArrayExpr(indent+1, values)})
	}
	if len(field.GetEnumValues()) > 0 {
		values := make([]string, 0, len(field.GetEnumValues()))
		for _, value := range field.GetEnumValues() {
			values = append(values, swiftEnumValueDocExpr(value, indent+2))
		}
		assignments = append(assignments, generatedField{name: "enumValues", value: swiftArrayExpr(indent+1, values)})
	}
	if field.GetRequired() {
		assignments = append(assignments, generatedField{name: "required", value: "true"})
	}
	if field.GetExample() != "" {
		assignments = append(assignments, generatedField{name: "example", value: sourceStringLiteral(field.GetExample())})
	}
	return swiftWithExpr(indent, "Holons_V1_FieldDoc", assignments)
}

func swiftEnumValueDocExpr(value *holonsv1.EnumValueDoc, indent int) string {
	assignments := make([]generatedField, 0, 3)
	if value.GetName() != "" {
		assignments = append(assignments, generatedField{name: "name", value: sourceStringLiteral(value.GetName())})
	}
	if value.GetNumber() != 0 {
		assignments = append(assignments, generatedField{name: "number", value: fmt.Sprintf("%d", value.GetNumber())})
	}
	if value.GetDescription() != "" {
		assignments = append(assignments, generatedField{name: "description_p", value: sourceStringLiteral(value.GetDescription())})
	}
	return swiftWithExpr(indent, "Holons_V1_EnumValueDoc", assignments)
}

func swiftWithExpr(indent int, typeName string, assignments []generatedField) string {
	if len(assignments) == 0 {
		return typeName + "()"
	}
	var buf strings.Builder
	buf.WriteString(typeName)
	buf.WriteString(".with {\n")
	for _, assignment := range assignments {
		buf.WriteString(genericIndent("  ", indent+1))
		buf.WriteString("$0.")
		buf.WriteString(assignment.name)
		buf.WriteString(" = ")
		buf.WriteString(assignment.value)
		buf.WriteString("\n")
	}
	buf.WriteString(genericIndent("  ", indent))
	buf.WriteString("}")
	return buf.String()
}

func swiftArrayExpr(indent int, values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for _, value := range values {
		buf.WriteString(genericIndent("  ", indent+1))
		buf.WriteString(value)
		buf.WriteString(",\n")
	}
	buf.WriteString(genericIndent("  ", indent))
	buf.WriteString("]")
	return buf.String()
}

func swiftFieldLabelLiteral(value holonsv1.FieldLabel) string {
	switch value {
	case holonsv1.FieldLabel_FIELD_LABEL_UNSPECIFIED:
		return ".unspecified"
	case holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL:
		return ".optional"
	case holonsv1.FieldLabel_FIELD_LABEL_REPEATED:
		return ".repeated"
	case holonsv1.FieldLabel_FIELD_LABEL_MAP:
		return ".map"
	case holonsv1.FieldLabel_FIELD_LABEL_REQUIRED:
		return ".required"
	default:
		return fmt.Sprintf("Holons_V1_FieldLabel(rawValue: %d) ?? .unspecified", value)
	}
}
