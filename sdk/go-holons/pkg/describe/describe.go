// Package describe provides build-time proto parsing helpers and runtime
// registration for a static HolonMeta response.
package describe

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/identity"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/types/descriptorpb"
)

const holonMetaServiceName = "holons.v1.HolonMeta"

type metaServer struct {
	holonsv1.UnimplementedHolonMetaServer
	response *holonsv1.DescribeResponse
}

func (s *metaServer) Describe(context.Context, *holonsv1.DescribeRequest) (*holonsv1.DescribeResponse, error) {
	return s.response, nil
}

// BuildResponse parses the proto directory and holon identity into a HolonMeta response.
func BuildResponse(protoDir string, manifestPath string) (*holonsv1.DescribeResponse, error) {
	manifest, err := resolveManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	services, err := parseServices(protoDir)
	if err != nil {
		return nil, err
	}

	return &holonsv1.DescribeResponse{
		Manifest: manifest,
		Services: services,
	}, nil
}

func parseServices(protoDir string) ([]*holonsv1.ServiceDoc, error) {
	absDir, err := filepath.Abs(protoDir)
	if err != nil {
		return nil, fmt.Errorf("resolve proto dir %s: %w", protoDir, err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat proto dir %s: %w", absDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absDir)
	}

	relFiles, err := collectProtoFiles(absDir)
	if err != nil {
		return nil, err
	}
	if len(relFiles) == 0 {
		return nil, nil
	}

	schemaRoots := discoverProtoImportPaths(absDir)
	parseFiles := relFiles
	importPaths := append([]string(nil), schemaRoots...)
	collision := hasSharedProtoCollision(absDir, relFiles, schemaRoots[1:])
	if collision {
		parseFiles = prefixImportRoot(filepath.Base(absDir), relFiles)
		importPaths = append([]string{filepath.Dir(absDir)}, schemaRoots[1:]...)
		importPaths = append(importPaths, absDir)
	}

	parser := protoparse.Parser{
		ImportPaths:               importPaths,
		InferImportPaths:          true,
		IncludeSourceCodeInfo:     true,
		LookupImport:              desc.LoadFileDescriptor,
		AllowExperimentalEditions: true,
	}
	files, err := parser.ParseFiles(parseFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse proto files in %s: %w", absDir, err)
	}

	expandableFiles, err := collectExpandableFiles(absDir, schemaRoots, collision)
	if err != nil {
		return nil, err
	}

	return responseBuilder{expandableFiles: expandableFiles}.buildServices(files), nil
}

type responseBuilder struct {
	expandableFiles map[string]struct{}
}

func (b responseBuilder) buildServices(files []*desc.FileDescriptor) []*holonsv1.ServiceDoc {
	seen := make(map[string]bool)
	serviceSeen := make(map[string]bool)
	services := make([]*holonsv1.ServiceDoc, 0)
	for _, file := range files {
		for _, service := range b.buildServicesFromFile(file, seen) {
			if serviceSeen[service.GetName()] {
				continue
			}
			serviceSeen[service.GetName()] = true
			services = append(services, service)
		}
	}
	return services
}

func (b responseBuilder) buildServicesFromFile(file *desc.FileDescriptor, seen map[string]bool) []*holonsv1.ServiceDoc {
	if file == nil {
		return nil
	}

	name := filepath.ToSlash(file.GetName())
	if seen[name] {
		return nil
	}
	seen[name] = true

	services := make([]*holonsv1.ServiceDoc, 0, len(file.GetServices()))
	for _, dep := range file.GetDependencies() {
		services = append(services, b.buildServicesFromFile(dep, seen)...)
	}
	for _, service := range file.GetServices() {
		if service.GetFullyQualifiedName() == holonMetaServiceName {
			continue
		}
		services = append(services, b.buildService(service))
	}
	return services
}

func (b responseBuilder) buildService(service *desc.ServiceDescriptor) *holonsv1.ServiceDoc {
	meta := parseCommentBlock(sourceComments(service.GetSourceInfo()))
	methods := make([]*holonsv1.MethodDoc, 0, len(service.GetMethods()))
	for _, method := range service.GetMethods() {
		methods = append(methods, b.buildMethod(method))
	}

	return &holonsv1.ServiceDoc{
		Name:        service.GetFullyQualifiedName(),
		Description: meta.Description,
		Methods:     methods,
	}
}

func (b responseBuilder) buildMethod(method *desc.MethodDescriptor) *holonsv1.MethodDoc {
	meta := parseCommentBlock(sourceComments(method.GetSourceInfo()))
	return &holonsv1.MethodDoc{
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

func (b responseBuilder) buildFields(message *desc.MessageDescriptor, seen map[string]bool) []*holonsv1.FieldDoc {
	if message == nil {
		return nil
	}

	name := message.GetFullyQualifiedName()
	if seen[name] {
		return nil
	}

	nextSeen := cloneSeen(seen)
	nextSeen[name] = true

	fields := make([]*holonsv1.FieldDoc, 0, len(message.GetFields()))
	for _, field := range message.GetFields() {
		fields = append(fields, b.buildField(field, nextSeen))
	}
	return fields
}

func (b responseBuilder) buildField(field *desc.FieldDescriptor, seen map[string]bool) *holonsv1.FieldDoc {
	meta := parseCommentBlock(sourceComments(field.GetSourceInfo()))
	doc := &holonsv1.FieldDoc{
		Name:        field.GetName(),
		Type:        descriptorTypeName(field),
		Number:      int32(field.GetNumber()),
		Description: meta.Description,
		Label:       fieldLabel(field),
		Required:    meta.Required,
		Example:     meta.Example,
	}

	if field.IsMap() {
		doc.MapKeyType = descriptorTypeName(field.GetMapKeyType())
		doc.MapValueType = descriptorTypeName(field.GetMapValueType())
		if enumType := field.GetMapValueType().GetEnumType(); enumType != nil && b.shouldExpand(enumType.GetFile().GetName()) {
			doc.EnumValues = buildEnumValues(enumType)
		}
		if msgType := field.GetMapValueType().GetMessageType(); msgType != nil && !msgType.IsMapEntry() && b.shouldExpand(msgType.GetFile().GetName()) {
			doc.NestedFields = b.buildFields(msgType, seen)
		}
		return doc
	}

	if enumType := field.GetEnumType(); enumType != nil && b.shouldExpand(enumType.GetFile().GetName()) {
		doc.EnumValues = buildEnumValues(enumType)
	}

	if msgType := field.GetMessageType(); msgType != nil && !msgType.IsMapEntry() && b.shouldExpand(msgType.GetFile().GetName()) {
		doc.NestedFields = b.buildFields(msgType, seen)
	}

	return doc
}

func buildEnumValues(enumType *desc.EnumDescriptor) []*holonsv1.EnumValueDoc {
	values := make([]*holonsv1.EnumValueDoc, 0, len(enumType.GetValues()))
	for _, value := range enumType.GetValues() {
		meta := parseCommentBlock(sourceComments(value.GetSourceInfo()))
		values = append(values, &holonsv1.EnumValueDoc{
			Name:        value.GetName(),
			Number:      int32(value.GetNumber()),
			Description: meta.Description,
		})
	}
	return values
}

func (b responseBuilder) shouldExpand(fileName string) bool {
	_, ok := b.expandableFiles[filepath.ToSlash(fileName)]
	return ok
}

func collectExpandableFiles(primaryRoot string, roots []string, collision bool) (map[string]struct{}, error) {
	expandableFiles := make(map[string]struct{})
	cleanPrimaryRoot := filepath.Clean(primaryRoot)

	for _, root := range roots {
		relFiles, err := collectProtoFiles(root)
		if err != nil {
			return nil, err
		}
		for _, rel := range relFiles {
			expandableFiles[filepath.ToSlash(rel)] = struct{}{}
		}
		if collision && filepath.Clean(root) == cleanPrimaryRoot {
			for _, rel := range prefixImportRoot(filepath.Base(cleanPrimaryRoot), relFiles) {
				expandableFiles[filepath.ToSlash(rel)] = struct{}{}
			}
		}
	}

	return expandableFiles, nil
}

func collectProtoFiles(dir string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(entry.Name()) != ".proto" {
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

func resolveManifest(manifestPath string) (*holonsv1.HolonManifest, error) {
	trimmed := strings.TrimSpace(manifestPath)
	if trimmed == "" {
		return nil, errors.New("manifest path is required")
	}

	info, err := os.Stat(trimmed)
	if err == nil && info.IsDir() {
		resolved, resolveErr := identity.Resolve(trimmed)
		if resolveErr != nil {
			return nil, resolveErr
		}
		return protoManifestFromResolved(resolved), nil
	}

	switch filepath.Base(trimmed) {
	case identity.ProtoManifestFileName:
		resolved, resolveErr := identity.ResolveProtoFile(trimmed)
		if resolveErr != nil {
			return nil, resolveErr
		}
		return protoManifestFromResolved(resolved), nil
	default:
		return nil, fmt.Errorf("%s is not a %s file", trimmed, identity.ProtoManifestFileName)
	}
}

func protoManifestFromResolved(resolved *identity.ResolvedManifest) *holonsv1.HolonManifest {
	if resolved == nil {
		return nil
	}

	manifest := &holonsv1.HolonManifest{
		Identity: &holonsv1.HolonManifest_Identity{
			Schema:     "holon/v1",
			Uuid:       resolved.Identity.UUID,
			GivenName:  resolved.Identity.GivenName,
			FamilyName: resolved.Identity.FamilyName,
			Motto:      resolved.Identity.Motto,
			Composer:   resolved.Identity.Composer,
			Status:     resolved.Identity.Status,
			Born:       resolved.Identity.Born,
			Version:    resolved.Identity.Version,
			Aliases:    append([]string(nil), resolved.Identity.Aliases...),
		},
		Description: resolved.Description,
		Lang:        resolved.Identity.Lang,
		Kind:        resolved.Kind,
	}

	if len(resolved.Skills) > 0 {
		manifest.Skills = make([]*holonsv1.HolonManifest_Skill, 0, len(resolved.Skills))
		for _, skill := range resolved.Skills {
			manifest.Skills = append(manifest.Skills, &holonsv1.HolonManifest_Skill{
				Name:        skill.Name,
				Description: skill.Description,
				When:        skill.When,
				Steps:       append([]string(nil), skill.Steps...),
			})
		}
	}

	if len(resolved.Sequences) > 0 {
		manifest.Sequences = make([]*holonsv1.HolonManifest_Sequence, 0, len(resolved.Sequences))
		for _, sequence := range resolved.Sequences {
			params := make([]*holonsv1.HolonManifest_Sequence_Param, 0, len(sequence.Params))
			for _, param := range sequence.Params {
				params = append(params, &holonsv1.HolonManifest_Sequence_Param{
					Name:        param.Name,
					Description: param.Description,
					Required:    param.Required,
					Default:     param.Default,
				})
			}
			manifest.Sequences = append(manifest.Sequences, &holonsv1.HolonManifest_Sequence{
				Name:        sequence.Name,
				Description: sequence.Description,
				Params:      params,
				Steps:       append([]string(nil), sequence.Steps...),
			})
		}
	}

	if resolved.BuildRunner != "" || resolved.BuildMain != "" || len(resolved.MemberPaths) > 0 {
		manifest.Build = &holonsv1.HolonManifest_Build{
			Runner: resolved.BuildRunner,
			Main:   resolved.BuildMain,
		}
	}
	if resolved.ArtifactBinary != "" || resolved.ArtifactPrimary != "" {
		manifest.Artifacts = &holonsv1.HolonManifest_Artifacts{
			Binary:  resolved.ArtifactBinary,
			Primary: resolved.ArtifactPrimary,
		}
	}
	if len(resolved.RequiredFiles) > 0 {
		manifest.Requires = &holonsv1.HolonManifest_Requires{
			Files: append([]string(nil), resolved.RequiredFiles...),
		}
	}
	return manifest
}

func discoverProtoImportPaths(protoDir string) []string {
	cleanProtoDir := filepath.Clean(protoDir)
	roots := []string{cleanProtoDir}
	seen := map[string]struct{}{cleanProtoDir: {}}

	// Staged holons often carry shared imports in a local "_protos" tree.
	appendIfDir(&roots, seen, filepath.Join(cleanProtoDir, "_protos"))
	appendIfDir(&roots, seen, filepath.Join(cleanProtoDir, "recipes", "protos"))

	for current := filepath.Dir(cleanProtoDir); current != "" && current != filepath.Dir(current); current = filepath.Dir(current) {
		appendIfDir(&roots, seen, filepath.Join(current, "_protos"))
		candidate := filepath.Join(current, "recipes", "protos")
		appendIfDir(&roots, seen, candidate)
	}
	return roots
}

func appendIfDir(roots *[]string, seen map[string]struct{}, candidate string) {
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return
	}
	cleaned := filepath.Clean(candidate)
	if _, ok := seen[cleaned]; ok {
		return
	}
	seen[cleaned] = struct{}{}
	*roots = append(*roots, cleaned)
}

func hasSharedProtoCollision(protoDir string, relFiles []string, sharedRoots []string) bool {
	for _, rel := range relFiles {
		localPath := filepath.Join(protoDir, filepath.FromSlash(rel))
		localInfo, err := os.Stat(localPath)
		if err != nil || localInfo.IsDir() {
			continue
		}
		for _, root := range sharedRoots {
			sharedPath := filepath.Join(root, filepath.FromSlash(rel))
			sharedInfo, sharedErr := os.Stat(sharedPath)
			if sharedErr == nil && !sharedInfo.IsDir() {
				return true
			}
		}
	}
	return false
}

func prefixImportRoot(root string, relFiles []string) []string {
	prefixed := make([]string, 0, len(relFiles))
	for _, rel := range relFiles {
		prefixed = append(prefixed, filepath.ToSlash(filepath.Join(root, filepath.FromSlash(rel))))
	}
	return prefixed
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

func cloneSeen(seen map[string]bool) map[string]bool {
	next := make(map[string]bool, len(seen)+1)
	for key, value := range seen {
		next[key] = value
	}
	return next
}

func fieldLabel(field *desc.FieldDescriptor) holonsv1.FieldLabel {
	if field.IsMap() {
		return holonsv1.FieldLabel_FIELD_LABEL_MAP
	}
	if field.IsRepeated() {
		return holonsv1.FieldLabel_FIELD_LABEL_REPEATED
	}
	return holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL
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
