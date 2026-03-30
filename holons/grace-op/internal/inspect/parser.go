package inspect

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
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

	parser := protoparse.Parser{
		ImportPaths:               discoverProtoImportPaths(absDir),
		InferImportPaths:          true,
		IncludeSourceCodeInfo:     true,
		LookupImport:              desc.LoadFileDescriptor,
		LookupImportProto:         nil,
		AllowExperimentalEditions: true,
	}
	files, err := parser.ParseFiles(relFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse proto files in %s: %w", absDir, err)
	}

	inputFiles := make(map[string]struct{}, len(relFiles))
	for _, rel := range relFiles {
		inputFiles[filepath.ToSlash(rel)] = struct{}{}
	}

	builder := parserBuilder{inputFiles: inputFiles}
	document, methods := builder.buildCatalog(files)
	return &Catalog{
		Document: document,
		Methods:  methods,
	}, nil
}

type parserBuilder struct {
	inputFiles map[string]struct{}
}

func (b parserBuilder) buildCatalog(files []*desc.FileDescriptor) (*Document, []MethodBinding) {
	document := &Document{
		Services: make([]Service, 0),
	}
	methods := make([]MethodBinding, 0)
	fileSeen := make(map[string]bool)
	serviceSeen := make(map[string]bool)
	for _, file := range files {
		fileServices, fileMethods := b.buildCatalogFromFile(file, fileSeen, serviceSeen)
		document.Services = append(document.Services, fileServices...)
		methods = append(methods, fileMethods...)
	}
	return document, methods
}

func (b parserBuilder) buildCatalogFromFile(
	file *desc.FileDescriptor,
	fileSeen map[string]bool,
	serviceSeen map[string]bool,
) ([]Service, []MethodBinding) {
	if file == nil {
		return nil, nil
	}

	name := filepath.ToSlash(file.GetName())
	if fileSeen[name] {
		return nil, nil
	}
	fileSeen[name] = true

	services := make([]Service, 0, len(file.GetServices()))
	methods := make([]MethodBinding, 0)
	for _, dep := range file.GetDependencies() {
		depServices, depMethods := b.buildCatalogFromFile(dep, fileSeen, serviceSeen)
		services = append(services, depServices...)
		methods = append(methods, depMethods...)
	}
	for _, service := range file.GetServices() {
		if serviceSeen[service.GetFullyQualifiedName()] {
			continue
		}
		serviceSeen[service.GetFullyQualifiedName()] = true
		serviceDoc, serviceMethods := b.buildService(service)
		services = append(services, serviceDoc)
		methods = append(methods, serviceMethods...)
	}
	return services, methods
}

func (b parserBuilder) buildService(service *desc.ServiceDescriptor) (Service, []MethodBinding) {
	meta := parseCommentBlock(sourceComments(service.GetSourceInfo()))
	methods := make([]Method, 0, len(service.GetMethods()))
	bindings := make([]MethodBinding, 0, len(service.GetMethods()))
	for _, method := range service.GetMethods() {
		methodDoc := b.buildMethod(method)
		methods = append(methods, methodDoc)
		bindings = append(bindings, MethodBinding{
			ServiceName:      service.GetFullyQualifiedName(),
			ServiceShortName: ShortName(service.GetFullyQualifiedName()),
			Method:           methodDoc,
			Descriptor:       method.UnwrapMethod(),
		})
	}
	return Service{
		Name:        service.GetFullyQualifiedName(),
		Description: meta.Description,
		Methods:     methods,
	}, bindings
}

func (b parserBuilder) buildMethod(method *desc.MethodDescriptor) Method {
	meta := parseCommentBlock(sourceComments(method.GetSourceInfo()))
	return Method{
		Name:            method.GetName(),
		Description:     meta.Description,
		InputType:       method.GetInputType().GetFullyQualifiedName(),
		OutputType:      method.GetOutputType().GetFullyQualifiedName(),
		InputFields:     b.buildFields(method.GetInputType(), map[string]bool{}),
		OutputFields:    b.buildFields(method.GetOutputType(), map[string]bool{}),
		ClientStreaming: method.IsClientStreaming(),
		ServerStreaming: method.IsServerStreaming(),
		ExampleInput:    meta.Example,
	}
}

func (b parserBuilder) buildFields(message *desc.MessageDescriptor, seen map[string]bool) []Field {
	if message == nil {
		return nil
	}
	name := message.GetFullyQualifiedName()
	if seen[name] {
		return nil
	}
	nextSeen := cloneSeen(seen)
	nextSeen[name] = true

	out := make([]Field, 0, len(message.GetFields()))
	for _, field := range message.GetFields() {
		out = append(out, b.buildField(field, nextSeen))
	}
	return out
}

func (b parserBuilder) buildField(field *desc.FieldDescriptor, seen map[string]bool) Field {
	meta := parseCommentBlock(sourceComments(field.GetSourceInfo()))
	out := Field{
		Name:        field.GetName(),
		Type:        descriptorTypeName(field),
		Number:      field.GetNumber(),
		Description: meta.Description,
		Label:       fieldLabel(field),
		Required:    meta.Required,
		Example:     meta.Example,
	}

	if field.IsMap() {
		out.MapKeyType = descriptorTypeName(field.GetMapKeyType())
		out.MapValueType = descriptorTypeName(field.GetMapValueType())
		if enumType := field.GetMapValueType().GetEnumType(); enumType != nil && b.shouldExpand(enumType.GetFile().GetName()) {
			out.EnumValues = buildEnumValues(enumType)
		}
		if msgType := field.GetMapValueType().GetMessageType(); msgType != nil && !msgType.IsMapEntry() && b.shouldExpand(msgType.GetFile().GetName()) {
			out.NestedFields = b.buildFields(msgType, seen)
		}
		return out
	}

	if enumType := field.GetEnumType(); enumType != nil && b.shouldExpand(enumType.GetFile().GetName()) {
		out.EnumValues = buildEnumValues(enumType)
	}

	if msgType := field.GetMessageType(); msgType != nil && !msgType.IsMapEntry() && b.shouldExpand(msgType.GetFile().GetName()) {
		out.NestedFields = b.buildFields(msgType, seen)
	}

	return out
}

func buildEnumValues(enumType *desc.EnumDescriptor) []EnumValue {
	out := make([]EnumValue, 0, len(enumType.GetValues()))
	for _, value := range enumType.GetValues() {
		meta := parseCommentBlock(sourceComments(value.GetSourceInfo()))
		out = append(out, EnumValue{
			Name:        value.GetName(),
			Number:      value.GetNumber(),
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
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
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
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan proto dir %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
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
	Description string
	Required    bool
	Example     string
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
		Description: strings.Join(description, " "),
		Required:    required,
		Example:     strings.Join(examples, "\n"),
	}
}

func sourceComments(location *descriptorpb.SourceCodeInfo_Location) string {
	if location == nil {
		return ""
	}
	if leading := strings.TrimSpace(location.GetLeadingComments()); leading != "" {
		return leading
	}
	return strings.TrimSpace(location.GetTrailingComments())
}

func cloneSeen(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}

func fieldLabel(field *desc.FieldDescriptor) string {
	if field.IsMap() {
		return FieldLabelMap
	}
	if field.IsRepeated() {
		return FieldLabelRepeated
	}
	if field.IsRequired() {
		return FieldLabelRequired
	}
	return FieldLabelOptional
}

func descriptorTypeName(field *desc.FieldDescriptor) string {
	if field == nil {
		return ""
	}
	if field.IsMap() {
		return fmt.Sprintf("map<%s, %s>", descriptorTypeName(field.GetMapKeyType()), descriptorTypeName(field.GetMapValueType()))
	}
	if enumType := field.GetEnumType(); enumType != nil {
		return enumType.GetFullyQualifiedName()
	}
	if msgType := field.GetMessageType(); msgType != nil {
		return msgType.GetFullyQualifiedName()
	}

	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "double"
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float"
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return "int64"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return "uint64"
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return "int32"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return "fixed64"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return "fixed32"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "string"
	case descriptorpb.FieldDescriptorProto_TYPE_GROUP:
		return "group"
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "bytes"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return "uint32"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return "sfixed32"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return "sfixed64"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		return "sint32"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return "sint64"
	default:
		return strings.TrimPrefix(field.GetType().String(), "TYPE_")
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
