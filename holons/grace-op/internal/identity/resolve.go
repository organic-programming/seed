package identity

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	protosfs "github.com/organic-programming/grace-op/_protos"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	ProtoManifestFileName   = "holon.proto"
	manifestExtensionNumber = 50000
)

// Resolved describes the identity source discovered for a holon.
type Resolved struct {
	Identity              Identity
	SourcePath            string
	Description           string
	Skills                []ResolvedSkill
	Sequences             []ResolvedSequence
	HasContract           bool
	Kind                  string
	Transport             string
	Platforms             []string
	BuildRunner           string
	RequiredFiles         []string
	RequiredCommands      []string
	RequiredSDKPrebuilts  []string
	BuildMain             string
	BuildDefaults         *ResolvedRecipeDefaults
	BuildMembers          []ResolvedRecipeMember
	BuildTargets          map[string]ResolvedRecipeTarget
	BuildTemplates        []string
	BuildCodegenLanguages []string
	BeforeCommands        []ResolvedRecipeExec
	AfterCommands         []ResolvedRecipeExec
	MemberPaths           []string
	ArtifactBinary        string
	PrimaryArtifact       string
	DelegateCommands      []string
}

type ResolvedRecipeDefaults struct {
	Target string
	Mode   string
}

type ResolvedRecipeMember struct {
	ID   string
	Path string
	Type string
}

type ResolvedRecipeTarget struct {
	Steps []ResolvedRecipeStep
}

type ResolvedRecipeStep struct {
	BuildMember   string
	Parallel      bool
	Exec          *ResolvedRecipeExec
	Copy          *ResolvedRecipeCopy
	AssertFile    *ResolvedRecipeFile
	CopyArtifact  *ResolvedRecipeCopyArtifact
	CopyAllHolons *ResolvedRecipeCopyAllHolons
}

type ResolvedRecipeExec struct {
	Cwd  string
	Argv []string
}

type ResolvedRecipeCopy struct {
	From string
	To   string
}

type ResolvedRecipeFile struct {
	Path string
}

type ResolvedRecipeCopyArtifact struct {
	From string
	To   string
}

type ResolvedRecipeCopyAllHolons struct {
	To string
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

// Resolve discovers a holon identity from dir using holon.proto only.
func Resolve(dir string) (*Resolved, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve dir %s: %w", dir, err)
	}
	return resolveFromProto(absDir)
}

// ResolveFromProtoFile extracts a holon identity from a specific holon.proto.
func ResolveFromProtoFile(path string) (*Resolved, error) {
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
		if resolved, ok := extractResolved(fd); ok {
			resolved.SourcePath = absPath
			return resolved, nil
		}
	}

	return nil, fmt.Errorf("no manifest extension found in %s", absPath)
}

func resolveFromProto(absDir string) (*Resolved, error) {
	protoFiles, err := collectProtoFiles(absDir)
	if err != nil {
		return nil, err
	}
	if len(protoFiles) == 0 {
		return nil, fmt.Errorf("no %s files found in %s", ProtoManifestFileName, absDir)
	}

	files, err := parseProtoFiles(absDir, protoFiles)
	if err != nil {
		return nil, err
	}

	for _, fd := range files {
		if resolved, ok := extractResolved(fd); ok {
			resolved.SourcePath = filepath.Join(absDir, filepath.FromSlash(fd.Path()))
			return resolved, nil
		}
	}

	return nil, fmt.Errorf("no manifest extension found in %s under %s", ProtoManifestFileName, absDir)
}

func parseProtoFiles(baseDir string, relFiles []string) ([]protoreflect.FileDescriptor, error) {
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(protocompile.CompositeResolver{
			protocompile.ResolverFunc(embeddedProtoResolver),
			&protocompile.SourceResolver{ImportPaths: buildImportPaths(baseDir)},
		}),
	}

	compiled, err := compiler.Compile(context.Background(), relFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse proto files in %s: %w", baseDir, err)
	}
	files := make([]protoreflect.FileDescriptor, 0, len(compiled))
	for _, file := range compiled {
		files = append(files, file)
	}
	return files, nil
}

// embeddedProtoResolver resolves canonical protos from the binary so identity
// parsing does not require a _protos/ directory on disk.
func embeddedProtoResolver(path string) (protocompile.SearchResult, error) {
	data, err := protosfs.FS.ReadFile(path)
	if err != nil {
		return protocompile.SearchResult{}, protoregistry.NotFound
	}
	return protocompile.SearchResult{Source: bytes.NewReader(data)}, nil
}

func extractResolved(fd protoreflect.FileDescriptor) (*Resolved, bool) {
	opts, ok := fd.Options().(*descriptorpb.FileOptions)
	if !ok || opts == nil {
		return nil, false
	}

	manifest, ok := manifestFromFileOptions(fd, opts)
	if !ok {
		return nil, false
	}

	resolved := resolvedFromManifest(manifest)
	if resolved.Identity.GivenName == "" && resolved.Identity.FamilyName == "" {
		return nil, false
	}

	return resolved, true
}

func manifestFromFileOptions(fd protoreflect.FileDescriptor, opts *descriptorpb.FileOptions) (*holonsv1.HolonManifest, bool) {
	if opts == nil {
		return nil, false
	}
	if ext := findExtension(fd, manifestExtensionNumber); ext != nil {
		if manifest, ok := manifestFromReflectExtension(opts, ext); ok {
			return manifest, true
		}
	}
	return manifestFromGeneratedExtension(opts)
}

func manifestFromReflectExtension(opts *descriptorpb.FileOptions, ext protoreflect.FieldDescriptor) (manifest *holonsv1.HolonManifest, ok bool) {
	defer func() {
		if recover() != nil {
			manifest = nil
			ok = false
		}
	}()
	optsMsg := opts.ProtoReflect()
	if !optsMsg.Has(ext) {
		return nil, false
	}
	value := optsMsg.Get(ext)
	message := value.Message()
	if !message.IsValid() {
		return nil, false
	}
	data, err := proto.Marshal(message.Interface())
	if err != nil {
		return nil, false
	}
	manifest = &holonsv1.HolonManifest{}
	if err := proto.Unmarshal(data, manifest); err != nil {
		return nil, false
	}
	return manifest, true
}

func manifestFromGeneratedExtension(opts *descriptorpb.FileOptions) (manifest *holonsv1.HolonManifest, ok bool) {
	defer func() {
		if recover() != nil {
			manifest = nil
			ok = false
		}
	}()
	if !proto.HasExtension(opts, holonsv1.E_Manifest) {
		return nil, false
	}
	value := proto.GetExtension(opts, holonsv1.E_Manifest)
	if manifest, ok := value.(*holonsv1.HolonManifest); ok {
		return manifest, true
	}
	message, ok := value.(proto.Message)
	if !ok {
		return nil, false
	}
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, false
	}
	manifest = &holonsv1.HolonManifest{}
	if err := proto.Unmarshal(data, manifest); err != nil {
		return nil, false
	}
	return manifest, true
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

func resolvedFromManifest(manifest *holonsv1.HolonManifest) *Resolved {
	resolved := &Resolved{}
	resolved.Description = manifest.GetDescription()
	resolved.Identity.Lang = manifest.GetLang()
	resolved.Kind = manifest.GetKind()
	resolved.Platforms = manifest.GetPlatforms()
	resolved.Transport = manifest.GetTransport()

	if ident := manifest.GetIdentity(); ident != nil {
		resolved.Identity.Schema = ident.GetSchema()
		resolved.Identity.UUID = ident.GetUuid()
		resolved.Identity.GivenName = ident.GetGivenName()
		resolved.Identity.FamilyName = ident.GetFamilyName()
		resolved.Identity.Motto = ident.GetMotto()
		resolved.Identity.Composer = ident.GetComposer()
		resolved.Identity.Status = ident.GetStatus()
		resolved.Identity.Born = ident.GetBorn()
		resolved.Identity.Version = ident.GetVersion()
		resolved.Identity.Aliases = ident.GetAliases()
	}

	if build := manifest.GetBuild(); build != nil {
		resolved.BuildRunner = build.GetRunner()
		resolved.BuildMain = build.GetMain()
		if defaults := build.GetDefaults(); defaults != nil {
			resolved.BuildDefaults = &ResolvedRecipeDefaults{
				Target: defaults.GetTarget(),
				Mode:   defaults.GetMode(),
			}
		}
		resolved.BuildMembers = make([]ResolvedRecipeMember, 0)
		resolved.MemberPaths = make([]string, 0)
		for _, member := range build.GetMembers() {
			resolvedMember := ResolvedRecipeMember{
				ID:   member.GetId(),
				Path: member.GetPath(),
				Type: member.GetType(),
			}
			resolved.BuildMembers = append(resolved.BuildMembers, resolvedMember)
			if path := strings.TrimSpace(resolvedMember.Path); path != "" {
				resolved.MemberPaths = append(resolved.MemberPaths, path)
			}
		}
		if targets := build.GetTargets(); len(targets) > 0 {
			resolved.BuildTargets = make(map[string]ResolvedRecipeTarget, len(targets))
			for key, target := range targets {
				resolved.BuildTargets[key] = resolvedRecipeTargetFromManifest(target)
			}
		}
		resolved.BuildTemplates = build.GetTemplates()
		if codegen := build.GetCodegen(); codegen != nil {
			resolved.BuildCodegenLanguages = codegen.GetLanguages()
		}

		resolved.BeforeCommands = make([]ResolvedRecipeExec, 0)
		for _, hook := range build.GetBeforeCommands() {
			resolved.BeforeCommands = append(resolved.BeforeCommands, ResolvedRecipeExec{
				Cwd:  hook.GetCwd(),
				Argv: hook.GetArgv(),
			})
		}

		resolved.AfterCommands = make([]ResolvedRecipeExec, 0)
		for _, hook := range build.GetAfterCommands() {
			resolved.AfterCommands = append(resolved.AfterCommands, ResolvedRecipeExec{
				Cwd:  hook.GetCwd(),
				Argv: hook.GetArgv(),
			})
		}
	}

	if requires := manifest.GetRequires(); requires != nil {
		resolved.RequiredCommands = requires.GetCommands()
		resolved.RequiredFiles = requires.GetFiles()
		resolved.RequiredSDKPrebuilts = requires.GetSdkPrebuilts()
	}

	if artifacts := manifest.GetArtifacts(); artifacts != nil {
		resolved.ArtifactBinary = artifacts.GetBinary()
		resolved.PrimaryArtifact = artifacts.GetPrimary()
	}

	resolved.Skills = make([]ResolvedSkill, 0)
	for _, skill := range manifest.GetSkills() {
		resolved.Skills = append(resolved.Skills, ResolvedSkill{
			Name:        skill.GetName(),
			Description: skill.GetDescription(),
			When:        skill.GetWhen(),
			Steps:       trimNonEmptyStrings(skill.GetSteps()),
		})
	}
	resolved.HasContract = manifest.GetContract() != nil

	resolved.Sequences = make([]ResolvedSequence, 0)
	for _, sequence := range manifest.GetSequences() {
		params := make([]ResolvedSequenceParam, 0)
		for _, param := range sequence.GetParams() {
			params = append(params, ResolvedSequenceParam{
				Name:        param.GetName(),
				Description: param.GetDescription(),
				Required:    param.GetRequired(),
				Default:     param.GetDefault(),
			})
		}
		resolved.Sequences = append(resolved.Sequences, ResolvedSequence{
			Name:        sequence.GetName(),
			Description: sequence.GetDescription(),
			Params:      params,
			Steps:       trimNonEmptyStrings(sequence.GetSteps()),
		})
	}

	resolved.Platforms = compactStrings(resolved.Platforms)
	resolved.RequiredCommands = compactStrings(resolved.RequiredCommands)
	resolved.RequiredFiles = compactStrings(resolved.RequiredFiles)
	resolved.RequiredSDKPrebuilts = compactStrings(resolved.RequiredSDKPrebuilts)
	resolved.BuildCodegenLanguages = compactStrings(resolved.BuildCodegenLanguages)
	resolved.MemberPaths = compactStrings(resolved.MemberPaths)
	resolved.DelegateCommands = compactStrings(resolved.DelegateCommands)
	return resolved
}

func resolvedRecipeTargetFromManifest(target *holonsv1.HolonManifest_Build_Target) ResolvedRecipeTarget {
	resolved := ResolvedRecipeTarget{
		Steps: make([]ResolvedRecipeStep, 0),
	}
	for _, step := range target.GetSteps() {
		resolved.Steps = append(resolved.Steps, resolvedRecipeStepFromManifest(step))
	}
	return resolved
}

func resolvedRecipeStepFromManifest(step *holonsv1.HolonManifest_Step) ResolvedRecipeStep {
	resolved := ResolvedRecipeStep{
		BuildMember: step.GetBuildMember(),
		Parallel:    step.GetParallel(),
	}
	if exec := step.GetExec(); exec != nil {
		resolved.Exec = &ResolvedRecipeExec{
			Cwd:  exec.GetCwd(),
			Argv: exec.GetArgv(),
		}
	}
	if copyStep := step.GetCopy(); copyStep != nil {
		resolved.Copy = &ResolvedRecipeCopy{
			From: copyStep.GetFrom(),
			To:   copyStep.GetTo(),
		}
	}
	if assertFile := step.GetAssertFile(); assertFile != nil {
		resolved.AssertFile = &ResolvedRecipeFile{
			Path: assertFile.GetPath(),
		}
	}
	if copyArtifact := step.GetCopyArtifact(); copyArtifact != nil {
		resolved.CopyArtifact = &ResolvedRecipeCopyArtifact{
			From: copyArtifact.GetFrom(),
			To:   copyArtifact.GetTo(),
		}
	}
	if copyAllHolons := step.GetCopyAllHolons(); copyAllHolons != nil {
		resolved.CopyAllHolons = &ResolvedRecipeCopyAllHolons{
			To: copyAllHolons.GetTo(),
		}
	}
	return resolved
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
				name == "node_modules" || name == "vendor" || name == "gen" ||
				name == "build" || name == "testdata" {
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

func buildImportPaths(dir string) []string {
	cleanDir := filepath.Clean(dir)
	paths := []string{cleanDir}
	seen := map[string]struct{}{cleanDir: {}}

	for current := filepath.Dir(dir); current != "" && current != filepath.Dir(current); current = filepath.Dir(current) {
		cleanCurrent := filepath.Clean(current)
		if _, ok := seen[cleanCurrent]; !ok {
			paths = append(paths, cleanCurrent)
			seen[cleanCurrent] = struct{}{}
		}

		candidate := filepath.Join(current, "_protos")
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		cleanCandidate := filepath.Clean(candidate)
		if _, ok := seen[cleanCandidate]; ok {
			continue
		}
		paths = append(paths, cleanCandidate)
		seen[cleanCandidate] = struct{}{}
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

func trimNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
