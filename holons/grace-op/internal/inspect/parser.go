package inspect

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// ParseProtoDir parses all .proto files under protoDir and returns a normalized
// inspection document. Identity and skills are attached by the caller.
func ParseProtoDir(protoDir string) (*Document, error) {
	catalog, err := ParseCatalog(protoDir)
	if err != nil {
		return nil, err
	}
	return catalog.Document, nil
}

// Catalog is the parsed inspection document plus method bindings that can be
// used to translate JSON tool calls into dynamic gRPC invocations.
type Catalog struct {
	Document *Document
	Methods  []MethodBinding
}

// MethodBinding ties an inspect.Method back to its gRPC method descriptor.
type MethodBinding struct {
	ServiceName      string
	ServiceShortName string
	Method           Method
	Descriptor       protoreflect.MethodDescriptor
}

func (m MethodBinding) ToolName(slug string) string {
	return strings.TrimSpace(slug) + "." + m.ServiceShortName + "." + m.Method.Name
}

func (m MethodBinding) FullMethod() string {
	return "/" + m.ServiceName + "/" + m.Method.Name
}

// ParseCatalog parses all .proto files under protoDir and returns both the
// human-readable document and descriptor bindings for method invocation.
func ParseCatalog(protoDir string) (*Catalog, error) {
	absDir, err := filepath.Abs(protoDir)
	if err != nil {
		return nil, fmt.Errorf("resolve proto dir %s: %w", protoDir, err)
	}

	relFiles, err := collectProtoFiles(absDir)
	if err != nil {
		return nil, err
	}
	if len(relFiles) == 0 {
		return nil, fmt.Errorf("no .proto files found in %s", absDir)
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(protocompile.CompositeResolver{
			&protocompile.SourceResolver{ImportPaths: discoverProtoImportPaths(absDir)},
			protocompile.ResolverFunc(globalProtoResolver),
		}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	compiled, err := compiler.Compile(context.Background(), relFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse proto files in %s: %w", absDir, err)
	}
	files := make([]protoreflect.FileDescriptor, 0, len(compiled))
	for _, file := range compiled {
		files = append(files, file)
	}

	inputFiles := make(map[string]struct{}, len(relFiles))
	for _, rel := range relFiles {
		inputFiles[filepath.ToSlash(rel)] = struct{}{}
	}

	builder := parserBuilder{inputFiles: inputFiles}
	document, methods, err := builder.buildCatalog(files)
	if err != nil {
		return nil, err
	}
	return &Catalog{
		Document: document,
		Methods:  methods,
	}, nil
}

func globalProtoResolver(path string) (protocompile.SearchResult, error) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	return protocompile.SearchResult{Desc: fd}, nil
}

type parserBuilder struct {
	inputFiles map[string]struct{}
}

func (b parserBuilder) buildCatalog(files []protoreflect.FileDescriptor) (*Document, []MethodBinding, error) {
	document := &Document{
		Services: make([]Service, 0),
	}
	methods := make([]MethodBinding, 0)
	fileSeen := make(map[string]bool)
	serviceSeen := make(map[string]bool)
	for _, file := range files {
		fileServices, fileMethods, err := b.buildCatalogFromFile(file, fileSeen, serviceSeen)
		if err != nil {
			return nil, nil, err
		}
		document.Services = append(document.Services, fileServices...)
		methods = append(methods, fileMethods...)
	}
	return document, methods, nil
}

func (b parserBuilder) buildCatalogFromFile(
	file protoreflect.FileDescriptor,
	fileSeen map[string]bool,
	serviceSeen map[string]bool,
) ([]Service, []MethodBinding, error) {
	if file == nil {
		return nil, nil, nil
	}

	name := filepath.ToSlash(file.Path())
	if fileSeen[name] {
		return nil, nil, nil
	}
	fileSeen[name] = true

	servicesList := file.Services()
	services := make([]Service, 0, servicesList.Len())
	methods := make([]MethodBinding, 0)
	imports := file.Imports()
	for i := 0; i < imports.Len(); i++ {
		depServices, depMethods, err := b.buildCatalogFromFile(imports.Get(i).FileDescriptor, fileSeen, serviceSeen)
		if err != nil {
			return nil, nil, err
		}
		services = append(services, depServices...)
		methods = append(methods, depMethods...)
	}
	for i := 0; i < servicesList.Len(); i++ {
		service := servicesList.Get(i)
		serviceName := string(service.FullName())
		if serviceSeen[serviceName] {
			continue
		}
		serviceSeen[serviceName] = true
		serviceDoc, serviceMethods, err := b.buildService(service)
		if err != nil {
			return nil, nil, err
		}
		services = append(services, serviceDoc)
		methods = append(methods, serviceMethods...)
	}
	return services, methods, nil
}

func (b parserBuilder) buildService(service protoreflect.ServiceDescriptor) (Service, []MethodBinding, error) {
	meta := parseCommentBlock(descriptorComments(service))
	methodDescriptors := service.Methods()
	methods := make([]Method, 0, methodDescriptors.Len())
	bindings := make([]MethodBinding, 0, methodDescriptors.Len())
	for i := 0; i < methodDescriptors.Len(); i++ {
		method := methodDescriptors.Get(i)
		methodDoc, err := b.buildMethod(method)
		if err != nil {
			return Service{}, nil, err
		}
		methods = append(methods, methodDoc)
		bindings = append(bindings, MethodBinding{
			ServiceName:      string(service.FullName()),
			ServiceShortName: ShortName(string(service.FullName())),
			Method:           methodDoc,
			Descriptor:       method,
		})
	}
	return Service{
		Name:        string(service.FullName()),
		Description: meta.Description,
		Methods:     methods,
	}, bindings, nil
}

func (b parserBuilder) buildMethod(method protoreflect.MethodDescriptor) (Method, error) {
	meta := parseCommentBlock(descriptorComments(method))
	examples, err := parseMethodExamples(string(method.FullName()), meta.ExampleLines)
	if err != nil {
		return Method{}, err
	}
	return Method{
		Name:            string(method.Name()),
		Description:     meta.Description,
		InputType:       string(method.Input().FullName()),
		OutputType:      string(method.Output().FullName()),
		InputFields:     b.buildFields(method.Input(), map[string]bool{}),
		OutputFields:    b.buildFields(method.Output(), map[string]bool{}),
		ClientStreaming: method.IsStreamingClient(),
		ServerStreaming: method.IsStreamingServer(),
		Examples:        examples,
	}, nil
}

func (b parserBuilder) buildFields(message protoreflect.MessageDescriptor, seen map[string]bool) []Field {
	if message == nil {
		return nil
	}
	name := string(message.FullName())
	if seen[name] {
		return nil
	}
	nextSeen := cloneSeen(seen)
	nextSeen[name] = true

	fields := message.Fields()
	out := make([]Field, 0, fields.Len())
	for i := 0; i < fields.Len(); i++ {
		out = append(out, b.buildField(fields.Get(i), nextSeen))
	}
	return out
}

func (b parserBuilder) buildField(field protoreflect.FieldDescriptor, seen map[string]bool) Field {
	meta := parseCommentBlock(descriptorComments(field))
	out := Field{
		Name:        string(field.Name()),
		Type:        descriptorTypeName(field),
		Number:      int32(field.Number()),
		Description: meta.Description,
		Label:       fieldLabel(field),
		Required:    meta.Required,
		Example:     meta.Example,
	}

	if field.IsMap() {
		out.MapKeyType = descriptorTypeName(field.MapKey())
		out.MapValueType = descriptorTypeName(field.MapValue())
		if enumType := field.MapValue().Enum(); enumType != nil && b.shouldExpand(enumType.ParentFile().Path()) {
			out.EnumValues = buildEnumValues(enumType)
		}
		if msgType := field.MapValue().Message(); msgType != nil && !msgType.IsMapEntry() && b.shouldExpand(msgType.ParentFile().Path()) {
			out.NestedFields = b.buildFields(msgType, seen)
		}
		return out
	}

	if enumType := field.Enum(); enumType != nil && b.shouldExpand(enumType.ParentFile().Path()) {
		out.EnumValues = buildEnumValues(enumType)
	}

	if msgType := field.Message(); msgType != nil && !msgType.IsMapEntry() && b.shouldExpand(msgType.ParentFile().Path()) {
		out.NestedFields = b.buildFields(msgType, seen)
	}

	return out
}

func buildEnumValues(enumType protoreflect.EnumDescriptor) []EnumValue {
	values := enumType.Values()
	out := make([]EnumValue, 0, values.Len())
	for i := 0; i < values.Len(); i++ {
		value := values.Get(i)
		meta := parseCommentBlock(descriptorComments(value))
		out = append(out, EnumValue{
			Name:        string(value.Name()),
			Number:      int32(value.Number()),
			Description: meta.Description,
		})
	}
	return out
}

func (b parserBuilder) shouldExpand(fileName string) bool {
	_, ok := b.inputFiles[filepath.ToSlash(fileName)]
	return ok
}

func collectProtoFiles(dir string) ([]string, error) {
	files := make([]string, 0)
	canonicalSeen := make(map[string]struct{})
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != dir {
				switch d.Name() {
				case "node_modules", "vendor", "build", "testdata":
					return filepath.SkipDir
				}
			}
			if strings.HasPrefix(d.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) != ".proto" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		canonical := canonicalProtoScanPath(rel)
		if _, ok := canonicalSeen[canonical]; ok {
			return nil
		}
		canonicalSeen[canonical] = struct{}{}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan proto dir %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

func canonicalProtoScanPath(rel string) string {
	switch filepath.ToSlash(rel) {
	case "xds/xds/data/orca/v3/orca_load_report.proto":
		return "xds/data/orca/v3/orca_load_report.proto"
	default:
		return filepath.ToSlash(rel)
	}
}

func discoverProtoImportPaths(protoDir string) []string {
	cleanProtoDir := filepath.Clean(protoDir)
	paths := []string{cleanProtoDir}
	seen := map[string]struct{}{cleanProtoDir: {}}

	for current := filepath.Dir(protoDir); current != "" && current != filepath.Dir(current); current = filepath.Dir(current) {
		appendImportDir(&paths, seen, filepath.Join(current, "_protos"))
		appendImportDir(&paths, seen, filepath.Join(current, "recipes", "protos"))
	}

	return paths
}

func appendImportDir(paths *[]string, seen map[string]struct{}, candidate string) {
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return
	}

	cleaned := filepath.Clean(candidate)
	if _, ok := seen[cleaned]; ok {
		return
	}

	seen[cleaned] = struct{}{}
	*paths = append(*paths, cleaned)
}

type commentMeta struct {
	Description  string
	Required     bool
	Example      string
	ExampleLines []string
}

func parseCommentBlock(raw string) commentMeta {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	description := make([]string, 0, len(lines))
	examples := make([]string, 0, 1)
	required := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case line == "@required":
			required = true
		case strings.HasPrefix(line, "@example"):
			example := strings.TrimSpace(strings.TrimPrefix(line, "@example"))
			if example != "" {
				examples = append(examples, example)
			}
		default:
			description = append(description, line)
		}
	}

	return commentMeta{
		Description:  strings.Join(description, " "),
		Required:     required,
		Example:      strings.Join(examples, "\n"),
		ExampleLines: append([]string(nil), examples...),
	}
}

func parseMethodExamples(methodName string, values []string) ([][]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	out := make([][]string, 0, len(values))
	for _, value := range values {
		tokens := parseExampleTokens(value)
		if len(tokens) == 0 {
			continue
		}
		if strings.Contains(tokens[0], "'") {
			return nil, fmt.Errorf("method %s: @example JSON payload must not contain single quote", methodName)
		}
		out = append(out, tokens)
	}
	return out, nil
}

// parseExampleTokens parses a single @example annotation value into an ordered
// list of shell tokens. The first token is the JSON payload when present,
// captured via balanced object/array parsing; subsequent tokens are split on
// ASCII whitespace.
func parseExampleTokens(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	tokens := make([]string, 0, 2)
	if value[0] == '{' || value[0] == '[' {
		end := balancedJSONPrefixEnd(value)
		if end < 0 {
			return []string{value}
		}
		tokens = append(tokens, strings.TrimSpace(value[:end]))
		value = strings.TrimSpace(value[end:])
	}

	for _, part := range strings.Fields(value) {
		part = strings.TrimSpace(part)
		if part != "" {
			tokens = append(tokens, part)
		}
	}
	return tokens
}

func balancedJSONPrefixEnd(value string) int {
	if value == "" {
		return -1
	}

	stack := []byte{value[0]}
	inString := false
	escaped := false

	for i := 1; i < len(value); i++ {
		ch := value[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}', ']':
			if len(stack) == 0 || ch != matchingClose(stack[len(stack)-1]) {
				return -1
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1
			}
		}
	}

	return -1
}

func matchingClose(open byte) byte {
	if open == '{' {
		return '}'
	}
	return ']'
}

func descriptorComments(desc protoreflect.Descriptor) string {
	if desc == nil || desc.ParentFile() == nil {
		return ""
	}
	location := desc.ParentFile().SourceLocations().ByDescriptor(desc)
	if leading := strings.TrimSpace(location.LeadingComments); leading != "" {
		return leading
	}
	return strings.TrimSpace(location.TrailingComments)
}

func cloneSeen(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}

func fieldLabel(field protoreflect.FieldDescriptor) string {
	if field.IsMap() {
		return FieldLabelMap
	}
	if field.IsList() {
		return FieldLabelRepeated
	}
	if field.Cardinality() == protoreflect.Required {
		return FieldLabelRequired
	}
	return FieldLabelOptional
}

func descriptorTypeName(field protoreflect.FieldDescriptor) string {
	if field == nil {
		return ""
	}
	if field.IsMap() {
		return fmt.Sprintf("map<%s, %s>", descriptorTypeName(field.MapKey()), descriptorTypeName(field.MapValue()))
	}
	if enumType := field.Enum(); enumType != nil {
		return string(enumType.FullName())
	}
	if msgType := field.Message(); msgType != nil {
		return string(msgType.FullName())
	}

	switch field.Kind() {
	case protoreflect.DoubleKind:
		return "double"
	case protoreflect.FloatKind:
		return "float"
	case protoreflect.Int64Kind:
		return "int64"
	case protoreflect.Uint64Kind:
		return "uint64"
	case protoreflect.Int32Kind:
		return "int32"
	case protoreflect.Fixed64Kind:
		return "fixed64"
	case protoreflect.Fixed32Kind:
		return "fixed32"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.GroupKind:
		return "group"
	case protoreflect.BytesKind:
		return "bytes"
	case protoreflect.Uint32Kind:
		return "uint32"
	case protoreflect.Sfixed32Kind:
		return "sfixed32"
	case protoreflect.Sfixed64Kind:
		return "sfixed64"
	case protoreflect.Sint32Kind:
		return "sint32"
	case protoreflect.Sint64Kind:
		return "sint64"
	default:
		return field.Kind().String()
	}
}

// ShortName returns the terminal identifier of a fully qualified proto symbol.
func ShortName(name string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(name), ".")
	if trimmed == "" {
		return ""
	}
	if idx := strings.LastIndex(trimmed, "."); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}
