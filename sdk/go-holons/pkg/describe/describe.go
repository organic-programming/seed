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

	"github.com/bufbuild/protocompile"
	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/identity"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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
	primaryRelFiles := primaryProtoFiles(relFiles)
	if len(primaryRelFiles) == 0 {
		return nil, nil
	}

	schemaRoots := discoverProtoImportPaths(absDir)
	parseFiles := primaryRelFiles
	importPaths := append([]string(nil), schemaRoots...)
	collision := hasSharedProtoCollision(absDir, primaryRelFiles, schemaRoots[1:])
	if collision {
		parseFiles = prefixImportRoot(filepath.Base(absDir), primaryRelFiles)
		importPaths = append([]string{filepath.Dir(absDir)}, schemaRoots[1:]...)
		importPaths = append(importPaths, absDir)
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(protocompile.CompositeResolver{
			&protocompile.SourceResolver{ImportPaths: importPaths},
			protocompile.ResolverFunc(globalProtoResolver),
		}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	compiled, err := compiler.Compile(context.Background(), parseFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse proto files in %s: %w", absDir, err)
	}
	files := make([]protoreflect.FileDescriptor, 0, len(compiled))
	for _, file := range compiled {
		files = append(files, file)
	}

	expandableFiles, err := collectExpandableFiles(absDir, schemaRoots, collision)
	if err != nil {
		return nil, err
	}

	return responseBuilder{expandableFiles: expandableFiles}.buildServices(files), nil
}

func globalProtoResolver(path string) (protocompile.SearchResult, error) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	return protocompile.SearchResult{Desc: fd}, nil
}

type responseBuilder struct {
	expandableFiles map[string]struct{}
}

func (b responseBuilder) buildServices(files []protoreflect.FileDescriptor) []*holonsv1.ServiceDoc {
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

func (b responseBuilder) buildServicesFromFile(file protoreflect.FileDescriptor, seen map[string]bool) []*holonsv1.ServiceDoc {
	if file == nil {
		return nil
	}

	name := filepath.ToSlash(file.Path())
	if seen[name] {
		return nil
	}
	seen[name] = true

	serviceDescriptors := file.Services()
	services := make([]*holonsv1.ServiceDoc, 0, serviceDescriptors.Len())
	imports := file.Imports()
	for i := 0; i < imports.Len(); i++ {
		services = append(services, b.buildServicesFromFile(imports.Get(i).FileDescriptor, seen)...)
	}
	for i := 0; i < serviceDescriptors.Len(); i++ {
		service := serviceDescriptors.Get(i)
		if service.FullName() == holonMetaServiceName {
			continue
		}
		services = append(services, b.buildService(service))
	}
	return services
}

func (b responseBuilder) buildService(service protoreflect.ServiceDescriptor) *holonsv1.ServiceDoc {
	meta := parseCommentBlock(descriptorComments(service))
	methodDescriptors := service.Methods()
	methods := make([]*holonsv1.MethodDoc, 0, methodDescriptors.Len())
	for i := 0; i < methodDescriptors.Len(); i++ {
		method := methodDescriptors.Get(i)
		methods = append(methods, b.buildMethod(method))
	}

	return &holonsv1.ServiceDoc{
		Name:        string(service.FullName()),
		Description: meta.Description,
		Methods:     methods,
	}
}

func (b responseBuilder) buildMethod(method protoreflect.MethodDescriptor) *holonsv1.MethodDoc {
	meta := parseCommentBlock(descriptorComments(method))
	return &holonsv1.MethodDoc{
		Name:            string(method.Name()),
		Description:     meta.Description,
		InputType:       string(method.Input().FullName()),
		OutputType:      string(method.Output().FullName()),
		InputFields:     b.buildFields(method.Input(), map[string]bool{}),
		OutputFields:    b.buildFields(method.Output(), map[string]bool{}),
		ClientStreaming: method.IsStreamingClient(),
		ServerStreaming: method.IsStreamingServer(),
		ExampleInput:    meta.Example,
	}
}

func (b responseBuilder) buildFields(message protoreflect.MessageDescriptor, seen map[string]bool) []*holonsv1.FieldDoc {
	if message == nil {
		return nil
	}

	name := string(message.FullName())
	if seen[name] {
		return nil
	}

	nextSeen := cloneSeen(seen)
	nextSeen[name] = true

	fieldDescriptors := message.Fields()
	fields := make([]*holonsv1.FieldDoc, 0, fieldDescriptors.Len())
	for i := 0; i < fieldDescriptors.Len(); i++ {
		fields = append(fields, b.buildField(fieldDescriptors.Get(i), nextSeen))
	}
	return fields
}

func (b responseBuilder) buildField(field protoreflect.FieldDescriptor, seen map[string]bool) *holonsv1.FieldDoc {
	meta := parseCommentBlock(descriptorComments(field))
	doc := &holonsv1.FieldDoc{
		Name:        string(field.Name()),
		Type:        descriptorTypeName(field),
		Number:      int32(field.Number()),
		Description: meta.Description,
		Label:       fieldLabel(field),
		Required:    meta.Required,
		Example:     meta.Example,
	}

	if field.IsMap() {
		doc.MapKeyType = descriptorTypeName(field.MapKey())
		doc.MapValueType = descriptorTypeName(field.MapValue())
		if enumType := field.MapValue().Enum(); enumType != nil && b.shouldExpand(enumType.ParentFile().Path()) {
			doc.EnumValues = buildEnumValues(enumType)
		}
		if msgType := field.MapValue().Message(); msgType != nil && !msgType.IsMapEntry() && b.shouldExpand(msgType.ParentFile().Path()) {
			doc.NestedFields = b.buildFields(msgType, seen)
		}
		return doc
	}

	if enumType := field.Enum(); enumType != nil && b.shouldExpand(enumType.ParentFile().Path()) {
		doc.EnumValues = buildEnumValues(enumType)
	}

	if msgType := field.Message(); msgType != nil && !msgType.IsMapEntry() && b.shouldExpand(msgType.ParentFile().Path()) {
		doc.NestedFields = b.buildFields(msgType, seen)
	}

	return doc
}

func buildEnumValues(enumType protoreflect.EnumDescriptor) []*holonsv1.EnumValueDoc {
	enumValues := enumType.Values()
	values := make([]*holonsv1.EnumValueDoc, 0, enumValues.Len())
	for i := 0; i < enumValues.Len(); i++ {
		value := enumValues.Get(i)
		meta := parseCommentBlock(descriptorComments(value))
		values = append(values, &holonsv1.EnumValueDoc{
			Name:        string(value.Name()),
			Number:      int32(value.Number()),
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

func primaryProtoFiles(files []string) []string {
	primary := make([]string, 0, len(files))
	for _, file := range files {
		rel := filepath.ToSlash(file)
		if strings.HasPrefix(rel, "_protos/") || strings.HasPrefix(rel, "recipes/protos/") {
			continue
		}
		primary = append(primary, rel)
	}
	return primary
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

func cloneSeen(seen map[string]bool) map[string]bool {
	next := make(map[string]bool, len(seen)+1)
	for key, value := range seen {
		next[key] = value
	}
	return next
}

func fieldLabel(field protoreflect.FieldDescriptor) holonsv1.FieldLabel {
	if field.IsMap() {
		return holonsv1.FieldLabel_FIELD_LABEL_MAP
	}
	if field.IsList() {
		return holonsv1.FieldLabel_FIELD_LABEL_REPEATED
	}
	return holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL
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
