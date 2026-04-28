package identity

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	ProtoManifestFileName   = "holon.proto"
	manifestExtensionNumber = 50000
)

// ResolvedManifest is the manifest data discovered from holon.proto.
type ResolvedManifest struct {
	Identity        Identity
	SourcePath      string
	Description     string
	Kind            string
	Transport       string
	BuildRunner     string
	BuildMain       string
	ArtifactBinary  string
	ArtifactPrimary string
	RequiredFiles   []string
	MemberPaths     []string
	Skills          []ResolvedSkill
	Sequences       []ResolvedSequence
}

type ResolvedSkill struct {
	Name        string
	Description string
	When        string
	Steps       []string
}

type ResolvedSequence struct {
	Name        string
	Description string
	Params      []ResolvedSequenceParam
	Steps       []string
}

type ResolvedSequenceParam struct {
	Name        string
	Description string
	Required    bool
	Default     string
}

// Resolve discovers a holon's manifest data from holon.proto files under dir.
func Resolve(dir string) (*ResolvedManifest, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve dir %s: %w", dir, err)
	}

	resolved, err := resolveFromProto(absDir)
	if err != nil {
		return nil, fmt.Errorf("resolve %s in %s: %w", ProtoManifestFileName, absDir, err)
	}
	return resolved, nil
}

// ResolveManifest preserves the original identity-only API.
func ResolveManifest(dir string) (Identity, string, error) {
	resolved, err := Resolve(dir)
	if err != nil {
		return Identity{}, "", err
	}
	return resolved.Identity, resolved.SourcePath, nil
}

// ResolveProtoFile extracts manifest data from a specific holon.proto file.
func ResolveProtoFile(path string) (*ResolvedManifest, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve proto path %s: %w", path, err)
	}
	if filepath.Base(absPath) != ProtoManifestFileName {
		return nil, fmt.Errorf("%s is not a %s file", absPath, ProtoManifestFileName)
	}

	files, err := parseProtoFiles(filepath.Dir(absPath), []string{filepath.Base(absPath)})
	if err != nil {
		return nil, err
	}

	for _, fd := range files {
		if resolved, ok := extractResolvedFromFileOptions(fd); ok {
			resolved.SourcePath = absPath
			return resolved, nil
		}
	}

	return nil, fmt.Errorf("no manifest extension found in %s", absPath)
}

func resolveFromProto(absDir string) (*ResolvedManifest, error) {
	protoFiles, err := collectProtoFiles(absDir)
	if err != nil {
		return nil, err
	}
	if len(protoFiles) == 0 {
		return nil, fmt.Errorf("no %s found in %s", ProtoManifestFileName, absDir)
	}

	var lastErr error
	for _, relPath := range protoFiles {
		resolved, err := ResolveProtoFile(filepath.Join(absDir, filepath.FromSlash(relPath)))
		if err == nil {
			return resolved, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no manifest extension found in %s files under %s", ProtoManifestFileName, absDir)
}

func parseProtoFiles(baseDir string, relFiles []string) ([]protoreflect.FileDescriptor, error) {
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(protocompile.CompositeResolver{
			&protocompile.SourceResolver{ImportPaths: buildImportPaths(baseDir)},
			protocompile.ResolverFunc(globalProtoResolver),
		}),
	}

	compiled, err := compiler.Compile(context.Background(), relFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse proto files: %w", err)
	}
	files := make([]protoreflect.FileDescriptor, 0, len(compiled))
	for _, file := range compiled {
		files = append(files, file)
	}
	return files, nil
}

func globalProtoResolver(path string) (protocompile.SearchResult, error) {
	_ = holonsv1.File_holons_v1_manifest_proto
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	return protocompile.SearchResult{Desc: fd}, nil
}

func extractResolvedFromFileOptions(fd protoreflect.FileDescriptor) (*ResolvedManifest, bool) {
	opts, ok := fd.Options().(*descriptorpb.FileOptions)
	if !ok || opts == nil {
		return nil, false
	}

	manifestExt := findExtension(fd, manifestExtensionNumber)
	if manifestExt == nil {
		return nil, false
	}

	optsMsg := opts.ProtoReflect()
	if !optsMsg.Has(manifestExt) {
		return nil, false
	}

	manifest := optsMsg.Get(manifestExt).Message()
	if !manifest.IsValid() {
		return nil, false
	}

	resolved := resolvedFromDynamic(manifest)
	if resolved.Identity.GivenName == "" && resolved.Identity.FamilyName == "" {
		return nil, false
	}

	return resolved, true
}

func findExtension(fd protoreflect.FileDescriptor, fieldNum int32) protoreflect.FieldDescriptor {
	seen := map[string]bool{}
	return findExtensionRecursive(fd, fieldNum, seen)
}

func findExtensionRecursive(fd protoreflect.FileDescriptor, fieldNum int32, seen map[string]bool) protoreflect.FieldDescriptor {
	if fd == nil || seen[fd.Path()] {
		return nil
	}
	seen[fd.Path()] = true

	extensions := fd.Extensions()
	for i := 0; i < extensions.Len(); i++ {
		ext := extensions.Get(i)
		if ext.Number() == protoreflect.FieldNumber(fieldNum) {
			return ext
		}
	}
	imports := fd.Imports()
	for i := 0; i < imports.Len(); i++ {
		if ext := findExtensionRecursive(imports.Get(i).FileDescriptor, fieldNum, seen); ext != nil {
			return ext
		}
	}
	return nil
}

func resolvedFromDynamic(manifest protoreflect.Message) *ResolvedManifest {
	resolved := &ResolvedManifest{}
	resolved.Description = dynString(manifest, 3)
	resolved.Identity.Lang = dynString(manifest, 4)
	resolved.Skills = resolvedSkillsFromDynamic(manifest)
	resolved.Kind = dynString(manifest, 7)
	resolved.Transport = dynString(manifest, 9)
	resolved.Sequences = resolvedSequencesFromDynamic(manifest)

	if identMsg := dynSubMessage(manifest, 1); identMsg != nil {
		resolved.Identity.UUID = dynString(identMsg, 2)
		resolved.Identity.GivenName = dynString(identMsg, 3)
		resolved.Identity.FamilyName = dynString(identMsg, 4)
		resolved.Identity.Motto = dynString(identMsg, 5)
		resolved.Identity.Composer = dynString(identMsg, 6)
		resolved.Identity.Clade = dynString(identMsg, 7)
		resolved.Identity.Status = dynString(identMsg, 8)
		resolved.Identity.Born = dynString(identMsg, 9)
		resolved.Identity.Version = dynString(identMsg, 10)
		resolved.Identity.Aliases = dynStringSlice(identMsg, 11)
	}

	if lineageMsg := dynSubMessage(manifest, 2); lineageMsg != nil {
		resolved.Identity.Parents = dynStringSlice(lineageMsg, 1)
		resolved.Identity.Reproduction = dynString(lineageMsg, 2)
		resolved.Identity.GeneratedBy = dynString(lineageMsg, 3)
	}

	if buildMsg := dynSubMessage(manifest, 10); buildMsg != nil {
		resolved.BuildRunner = dynString(buildMsg, 1)
		resolved.BuildMain = dynString(buildMsg, 2)
		for _, memberMsg := range dynSubMessages(buildMsg, 4) {
			if path := strings.TrimSpace(dynString(memberMsg, 2)); path != "" {
				resolved.MemberPaths = append(resolved.MemberPaths, path)
			}
		}
	}

	if requiresMsg := dynSubMessage(manifest, 11); requiresMsg != nil {
		resolved.RequiredFiles = dynStringSlice(requiresMsg, 2)
	}

	if artifactsMsg := dynSubMessage(manifest, 13); artifactsMsg != nil {
		resolved.ArtifactBinary = dynString(artifactsMsg, 1)
		resolved.ArtifactPrimary = dynString(artifactsMsg, 2)
	}

	resolved.RequiredFiles = compactStrings(resolved.RequiredFiles)
	resolved.MemberPaths = compactStrings(resolved.MemberPaths)
	return resolved
}

func resolvedSkillsFromDynamic(manifest protoreflect.Message) []ResolvedSkill {
	messages := dynSubMessages(manifest, 5)
	out := make([]ResolvedSkill, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		out = append(out, ResolvedSkill{
			Name:        strings.TrimSpace(dynString(message, 1)),
			Description: strings.TrimSpace(dynString(message, 2)),
			When:        strings.TrimSpace(dynString(message, 3)),
			Steps:       trimmedStrings(dynStringSliceRaw(message, 4)),
		})
	}
	return out
}

func resolvedSequencesFromDynamic(manifest protoreflect.Message) []ResolvedSequence {
	messages := dynSubMessages(manifest, 14)
	out := make([]ResolvedSequence, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}

		params := make([]ResolvedSequenceParam, 0, len(dynSubMessages(message, 3)))
		for _, param := range dynSubMessages(message, 3) {
			if param == nil {
				continue
			}
			params = append(params, ResolvedSequenceParam{
				Name:        strings.TrimSpace(dynString(param, 1)),
				Description: strings.TrimSpace(dynString(param, 2)),
				Required:    dynBool(param, 3),
				Default:     strings.TrimSpace(dynString(param, 4)),
			})
		}

		out = append(out, ResolvedSequence{
			Name:        strings.TrimSpace(dynString(message, 1)),
			Description: strings.TrimSpace(dynString(message, 2)),
			Params:      params,
			Steps:       trimmedStrings(dynStringSliceRaw(message, 4)),
		})
	}
	return out
}

func dynString(msg protoreflect.Message, fieldNum int) string {
	field := fieldByNumber(msg, fieldNum)
	if field == nil || !msg.Has(field) {
		return ""
	}
	return msg.Get(field).String()
}

func dynSubMessage(msg protoreflect.Message, fieldNum int) protoreflect.Message {
	field := fieldByNumber(msg, fieldNum)
	if field == nil || !msg.Has(field) {
		return nil
	}
	sub := msg.Get(field).Message()
	if !sub.IsValid() {
		return nil
	}
	return sub
}

func dynSubMessages(msg protoreflect.Message, fieldNum int) []protoreflect.Message {
	field := fieldByNumber(msg, fieldNum)
	if field == nil || !msg.Has(field) {
		return nil
	}

	list := msg.Get(field).List()
	out := make([]protoreflect.Message, 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		sub := list.Get(i).Message()
		if sub.IsValid() {
			out = append(out, sub)
		}
	}
	return out
}

func dynBool(msg protoreflect.Message, fieldNum int) bool {
	field := fieldByNumber(msg, fieldNum)
	if field == nil || !msg.Has(field) {
		return false
	}
	return msg.Get(field).Bool()
}

func dynStringSlice(msg protoreflect.Message, fieldNum int) []string {
	return compactStrings(dynStringSliceRaw(msg, fieldNum))
}

func dynStringSliceRaw(msg protoreflect.Message, fieldNum int) []string {
	field := fieldByNumber(msg, fieldNum)
	if field == nil || !msg.Has(field) {
		return nil
	}

	list := msg.Get(field).List()
	out := make([]string, 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		out = append(out, list.Get(i).String())
	}
	return out
}

func fieldByNumber(msg protoreflect.Message, fieldNum int) protoreflect.FieldDescriptor {
	if msg == nil || !msg.IsValid() {
		return nil
	}
	return msg.Descriptor().Fields().ByNumber(protoreflect.FieldNumber(fieldNum))
}

func collectProtoFiles(dir string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if (strings.HasPrefix(name, ".") && path != dir) ||
				name == "node_modules" || name == "vendor" || name == "gen" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != ProtoManifestFileName {
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
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func buildImportPaths(dir string) []string {
	paths := []string{dir}
	seen := map[string]struct{}{dir: {}}
	staged := make([]string, 0)

	// Walk up the tree to find source proto roots first. Staged .op/protos
	// can be stale after schema edits, so keep them as a fallback.
	for current := filepath.Dir(dir); current != "" && current != filepath.Dir(current); current = filepath.Dir(current) {
		candidates := []string{
			filepath.Join(current, "_protos"),
			filepath.Join(current, "seed", "holons", "grace-op", "_protos"),
		}

		for _, candidate := range candidates {
			info, err := os.Stat(candidate)
			if err != nil || !info.IsDir() {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			paths = append(paths, candidate)
			seen[candidate] = struct{}{}
		}

		staged = append(staged, filepath.Join(current, ".op", "protos"))
	}

	for _, candidate := range staged {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		paths = append(paths, candidate)
		seen[candidate] = struct{}{}
	}

	return paths
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

func trimmedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
