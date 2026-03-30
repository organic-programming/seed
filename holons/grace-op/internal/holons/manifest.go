package holons

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/organic-programming/grace-op/internal/identity"
)

const (
	SchemaV0       = "holon/v0"
	KindNative     = "native"
	KindWrapper    = "wrapper"
	KindComposite  = "composite"
	RunnerGoModule = "go-module"
	RunnerCMake    = "cmake"
	RunnerCargo    = "cargo"
	RunnerPython   = "python"
	RunnerDart     = "dart"
	RunnerRuby     = "ruby"
	RunnerSwiftPkg = "swift-package"
	RunnerFlutter  = "flutter"
	RunnerNPM      = "npm"
	RunnerGradle   = "gradle"
	RunnerDotnet   = "dotnet"
	RunnerQtCMake  = "qt-cmake"
	RunnerRecipe   = "recipe"
)

type Manifest struct {
	// Identity fields — present in holon.proto but not used by lifecycle.
	Schema      string
	UUID        string
	GivenName   string
	FamilyName  string
	Motto       string
	Composer    string
	Clade       string
	Status      string
	Born        string
	Version     string
	Lang        string
	Aliases     []string
	ProtoStatus string

	// Lineage fields.
	Parents      []string
	Reproduction string
	GeneratedBy  string

	// Description.
	Description string
	Skills      []Skill
	Sequences   []Sequence

	// Operational fields — used by lifecycle.
	Kind      string
	Transport string
	Platforms []string
	Build     BuildConfig
	Requires  Requires
	Delegates Delegates
	Artifacts ArtifactPaths

	// Contract fields — not used by lifecycle.
	Contract interface{}
}

type Skill struct {
	Name        string
	Description string
	When        string
	Steps       []string
}

type Sequence struct {
	Name        string
	Description string
	Params      []SequenceParam
	Steps       []string
}

type SequenceParam struct {
	Name        string
	Description string
	Required    bool
	Default     string
}

type BuildConfig struct {
	Runner    string
	Main      string
	Defaults  *RecipeDefaults
	Members   []RecipeMember
	Targets   map[string]RecipeTarget
	Templates []string
}

// RecipeDefaults provides default target and mode for recipe builds.
type RecipeDefaults struct {
	Target string
	Mode   string
}

// RecipeMember is a named build participant in a composite holon.
type RecipeMember struct {
	ID   string
	Path string
	Type string // "holon" or "component"
}

// RecipeTarget defines the build steps for a specific platform.
type RecipeTarget struct {
	Steps []RecipeStep
}

// RecipeStep is one step in a recipe build plan.
// Exactly one field should be set.
type RecipeStep struct {
	BuildMember  string
	Exec         *RecipeStepExec
	Copy         *RecipeStepCopy
	AssertFile   *RecipeStepFile
	CopyArtifact *RecipeStepCopyArtifact
}

// RecipeStepExec runs a command with an explicit argv and working directory.
type RecipeStepExec struct {
	Cwd  string
	Argv []string
}

// RecipeStepCopy copies a file from one manifest-relative path to another.
type RecipeStepCopy struct {
	From string
	To   string
}

// RecipeStepFile verifies a manifest-relative file exists.
type RecipeStepFile struct {
	Path string
}

type RecipeStepCopyArtifact struct {
	From string
	To   string
}

type Requires struct {
	Commands []string
	Files    []string
}

type Delegates struct {
	Commands []string
}

type ArtifactPaths struct {
	Binary  string
	Primary string
}

type LoadedManifest struct {
	Manifest Manifest
	Dir      string
	Path     string
	Name     string
}

var errProtoManifestNotFound = errors.New("no holon.proto found")

func (m *Manifest) ArtifactPath() string {
	if m == nil {
		return ""
	}
	if primary := strings.TrimSpace(m.Artifacts.Primary); primary != "" {
		return primary
	}
	return strings.TrimSpace(m.Artifacts.Binary)
}

func LoadManifest(dir string) (*LoadedManifest, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", dir, err)
	}

	loaded, err := loadProtoManifest(absDir)
	if err != nil {
		if errors.Is(err, errProtoManifestNotFound) {
			return nil, fmt.Errorf("no %s found in %s", identity.ProtoManifestFileName, absDir)
		}
		return nil, err
	}
	return loaded, nil
}

func loadProtoManifest(absDir string) (*LoadedManifest, error) {
	protoFiles, err := findProtoManifestFiles(absDir)
	if err != nil {
		return nil, err
	}
	if len(protoFiles) == 0 {
		return nil, errProtoManifestNotFound
	}

	var firstErr error
	for _, protoPath := range protoFiles {
		resolved, err := identity.ResolveFromProtoFile(protoPath)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		root := protoHolonDir(absDir, protoPath, resolved)
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if filepath.Clean(rootAbs) != filepath.Clean(absDir) {
			continue
		}

		loaded := &LoadedManifest{
			Manifest: manifestFromResolved(resolved),
			Dir:      absDir,
			Path:     resolved.SourcePath,
			Name:     filepath.Base(absDir),
		}

		if err := normalizeManifest(loaded); err != nil {
			return nil, err
		}
		if err := validateManifest(loaded); err != nil {
			return nil, err
		}
		return loaded, nil
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return nil, errProtoManifestNotFound
}

func findProtoManifestFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipDiscoveryDir(root, path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == identity.ProtoManifestFileName {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s for %s: %w", root, identity.ProtoManifestFileName, err)
	}
	slices.Sort(files)
	return files, nil
}

func manifestFromResolved(resolved *identity.Resolved) Manifest {
	if resolved == nil {
		return Manifest{}
	}

	var contract any
	if resolved.HasContract {
		contract = struct{}{}
	}

	return Manifest{
		Schema:       SchemaV0,
		UUID:         resolved.Identity.UUID,
		GivenName:    resolved.Identity.GivenName,
		FamilyName:   resolved.Identity.FamilyName,
		Motto:        resolved.Identity.Motto,
		Composer:     resolved.Identity.Composer,
		Clade:        resolved.Identity.Clade,
		Status:       resolved.Identity.Status,
		Born:         resolved.Identity.Born,
		Version:      resolved.Identity.Version,
		Lang:         resolved.Identity.Lang,
		ProtoStatus:  resolved.Identity.ProtoStatus,
		Parents:      slices.Clone(resolved.Identity.Parents),
		Reproduction: resolved.Identity.Reproduction,
		GeneratedBy:  resolved.Identity.GeneratedBy,
		Description:  resolved.Description,
		Skills:       manifestSkillsFromResolved(resolved.Skills),
		Sequences:    manifestSequencesFromResolved(resolved.Sequences),
		Kind:         resolved.Kind,
		Transport:    resolved.Transport,
		Platforms:    slices.Clone(resolved.Platforms),
		Build:        manifestBuildFromResolved(resolved),
		Requires: Requires{
			Commands: slices.Clone(resolved.RequiredCommands),
			Files:    slices.Clone(resolved.RequiredFiles),
		},
		Delegates: Delegates{
			Commands: slices.Clone(resolved.DelegateCommands),
		},
		Artifacts: ArtifactPaths{
			Binary:  resolved.ArtifactBinary,
			Primary: resolved.PrimaryArtifact,
		},
		Contract: contract,
	}
}

func manifestBuildFromResolved(resolved *identity.Resolved) BuildConfig {
	build := BuildConfig{
		Runner:    resolved.BuildRunner,
		Main:      resolved.BuildMain,
		Templates: resolved.BuildTemplates,
	}

	if resolved.BuildDefaults != nil {
		build.Defaults = &RecipeDefaults{
			Target: strings.TrimSpace(resolved.BuildDefaults.Target),
			Mode:   strings.TrimSpace(resolved.BuildDefaults.Mode),
		}
	}

	if len(resolved.BuildMembers) > 0 {
		build.Members = make([]RecipeMember, 0, len(resolved.BuildMembers))
		for _, member := range resolved.BuildMembers {
			build.Members = append(build.Members, RecipeMember{
				ID:   strings.TrimSpace(member.ID),
				Path: strings.TrimSpace(member.Path),
				Type: strings.TrimSpace(member.Type),
			})
		}
	}

	if len(resolved.BuildTargets) > 0 {
		build.Targets = make(map[string]RecipeTarget, len(resolved.BuildTargets))
		for name, target := range resolved.BuildTargets {
			steps := make([]RecipeStep, 0, len(target.Steps))
			for _, step := range target.Steps {
				recipeStep := RecipeStep{
					BuildMember: strings.TrimSpace(step.BuildMember),
				}
				if step.Exec != nil {
					recipeStep.Exec = &RecipeStepExec{
						Cwd:  strings.TrimSpace(step.Exec.Cwd),
						Argv: append([]string(nil), step.Exec.Argv...),
					}
				}
				if step.Copy != nil {
					recipeStep.Copy = &RecipeStepCopy{
						From: strings.TrimSpace(step.Copy.From),
						To:   strings.TrimSpace(step.Copy.To),
					}
				}
				if step.AssertFile != nil {
					recipeStep.AssertFile = &RecipeStepFile{
						Path: strings.TrimSpace(step.AssertFile.Path),
					}
				}
				if step.CopyArtifact != nil {
					recipeStep.CopyArtifact = &RecipeStepCopyArtifact{
						From: strings.TrimSpace(step.CopyArtifact.From),
						To:   strings.TrimSpace(step.CopyArtifact.To),
					}
				}
				steps = append(steps, recipeStep)
			}
			build.Targets[strings.TrimSpace(name)] = RecipeTarget{Steps: steps}
		}
	}

	return build
}

func manifestSkillsFromResolved(skills []identity.ResolvedSkill) []Skill {
	out := make([]Skill, 0, len(skills))
	for _, skill := range skills {
		out = append(out, Skill{
			Name:        strings.TrimSpace(skill.Name),
			Description: strings.TrimSpace(skill.Description),
			When:        strings.TrimSpace(skill.When),
			Steps:       append([]string(nil), skill.Steps...),
		})
	}
	return out
}

func manifestSequencesFromResolved(sequences []identity.ResolvedSequence) []Sequence {
	out := make([]Sequence, 0, len(sequences))
	for _, sequence := range sequences {
		params := make([]SequenceParam, 0, len(sequence.Params))
		for _, param := range sequence.Params {
			params = append(params, SequenceParam{
				Name:        strings.TrimSpace(param.Name),
				Description: strings.TrimSpace(param.Description),
				Required:    param.Required,
				Default:     strings.TrimSpace(param.Default),
			})
		}
		out = append(out, Sequence{
			Name:        strings.TrimSpace(sequence.Name),
			Description: strings.TrimSpace(sequence.Description),
			Params:      params,
			Steps:       append([]string(nil), sequence.Steps...),
		})
	}
	return out
}

func (m *LoadedManifest) SupportsCurrentPlatform() bool {
	return m.SupportsTarget(canonicalRuntimeTarget())
}

func (m *LoadedManifest) SupportsTarget(target string) bool {
	if m == nil || len(m.Manifest.Platforms) == 0 {
		return true
	}
	return slices.ContainsFunc(m.Manifest.Platforms, func(platform string) bool {
		return normalizePlatformName(platform) == normalizePlatformName(target)
	})
}

func (m *LoadedManifest) ResolveManifestPath(rel string) (string, error) {
	return resolveManifestPath(m.Dir, rel)
}

func (m *LoadedManifest) mustResolveManifestPath(rel string) string {
	resolved, err := m.ResolveManifestPath(rel)
	if err == nil {
		return resolved
	}
	return filepath.Join(m.Dir, filepath.FromSlash(rel))
}

func (m *LoadedManifest) HolonPackageDir() string {
	if m == nil {
		return ""
	}
	return filepath.Join(m.Dir, ".op", "build", m.Name+".holon")
}

func (m *LoadedManifest) BinaryPath() string {
	if binary := m.BinaryName(); binary != "" {
		return filepath.Join(m.HolonPackageDir(), "bin", runtimeArchitecture(), binary)
	}
	return ""
}

func (m *LoadedManifest) BinaryName() string {
	if m == nil {
		return ""
	}
	trimmed := strings.TrimSpace(m.Manifest.Artifacts.Binary)
	if trimmed == "" {
		return ""
	}
	return strings.TrimSpace(filepath.Base(trimmed))
}

// ArtifactPath returns the resolved launch/build artifact for the requested target.
// artifacts.primary takes precedence over artifacts.binary.
func (m *LoadedManifest) ArtifactPath(ctx BuildContext) string {
	if isAggregateBuildTarget(ctx.Target) {
		return ""
	}
	if strings.TrimSpace(m.Manifest.Artifacts.Primary) != "" {
		return m.mustResolveManifestPath(m.Manifest.ArtifactPath())
	}
	return m.HolonPackageDir()
}

// PrimaryArtifactPath returns the primary artifact path (success contract).
func (m *LoadedManifest) PrimaryArtifactPath(ctx BuildContext) string {
	return m.ArtifactPath(ctx)
}

func (m *LoadedManifest) OpRoot() string {
	return filepath.Join(m.Dir, ".op")
}

func runtimeArchitecture() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}

func (m *LoadedManifest) CMakeBuildDir() string {
	return filepath.Join(m.Dir, ".op", "build", "cmake")
}

func (m *LoadedManifest) GoMainPackage() string {
	if strings.TrimSpace(m.Manifest.Build.Main) != "" {
		return m.Manifest.Build.Main
	}
	return "./cmd/" + m.Name
}

func validateManifest(m *LoadedManifest) error {
	if m.Manifest.Schema != SchemaV0 {
		return fmt.Errorf("%s: schema must be %q", m.Path, SchemaV0)
	}

	switch m.Manifest.Kind {
	case KindNative, KindWrapper, KindComposite:
	default:
		return fmt.Errorf("%s: kind must be %q, %q, or %q", m.Path, KindNative, KindWrapper, KindComposite)
	}

	if !isSupportedRunner(m.Manifest.Build.Runner) {
		return fmt.Errorf("%s: build.runner must be one of %s", m.Path, supportedRunnerList())
	}

	// Artifact validation: binary required for native/wrapper, primary required for composite.
	hasBinary := strings.TrimSpace(m.Manifest.Artifacts.Binary) != ""
	hasPrimary := strings.TrimSpace(m.Manifest.Artifacts.Primary) != ""
	if hasBinary && hasPrimary {
		return fmt.Errorf("%s: artifacts.binary and artifacts.primary are mutually exclusive", m.Path)
	}

	switch m.Manifest.Kind {
	case KindNative, KindWrapper:
		if !hasBinary {
			return fmt.Errorf("%s: artifacts.binary is required for %s holons", m.Path, m.Manifest.Kind)
		}
	case KindComposite:
		if !hasPrimary {
			return fmt.Errorf("%s: artifacts.primary is required for composite holons", m.Path)
		}
	}
	if hasBinary {
		if err := validateBinaryName(m, m.Manifest.Artifacts.Binary); err != nil {
			return err
		}
	}
	if hasPrimary {
		if err := validateManifestRelativeField(m, "artifacts.primary", m.Manifest.Artifacts.Primary); err != nil {
			return err
		}
	}

	if m.Manifest.Kind != KindWrapper && len(m.Manifest.Delegates.Commands) > 0 {
		return fmt.Errorf("%s: delegates.commands is only valid for wrapper holons", m.Path)
	}

	// Recipe-specific validation.
	if m.Manifest.Build.Runner == RunnerRecipe {
		if err := validateRecipe(m); err != nil {
			return err
		}
	}

	for _, platform := range m.Manifest.Platforms {
		if !isValidPlatform(platform) {
			return fmt.Errorf("%s: unsupported platform %q", m.Path, platform)
		}
	}

	if err := validateList("requires.commands", m.Manifest.Requires.Commands); err != nil {
		return fmt.Errorf("%s: %w", m.Path, err)
	}
	if err := validateList("requires.files", m.Manifest.Requires.Files); err != nil {
		return fmt.Errorf("%s: %w", m.Path, err)
	}
	for _, requiredFile := range m.Manifest.Requires.Files {
		if err := validateManifestRelativeField(m, "requires.files", requiredFile); err != nil {
			return err
		}
	}
	if err := validateList("delegates.commands", m.Manifest.Delegates.Commands); err != nil {
		return fmt.Errorf("%s: %w", m.Path, err)
	}
	if err := validateSequences(m); err != nil {
		return err
	}

	return nil
}

// validateRecipe checks recipe-specific manifest constraints.
func validateRecipe(m *LoadedManifest) error {
	if len(m.Manifest.Build.Members) == 0 {
		return fmt.Errorf("%s: recipe runner requires at least one member", m.Path)
	}
	if len(m.Manifest.Build.Targets) == 0 {
		return fmt.Errorf("%s: recipe runner requires at least one target", m.Path)
	}

	memberIDs := make(map[string]bool, len(m.Manifest.Build.Members))
	memberTypes := make(map[string]string, len(m.Manifest.Build.Members))
	for _, member := range m.Manifest.Build.Members {
		if strings.TrimSpace(member.ID) == "" {
			return fmt.Errorf("%s: recipe member must have an id", m.Path)
		}
		if memberIDs[member.ID] {
			return fmt.Errorf("%s: duplicate recipe member id %q", m.Path, member.ID)
		}
		memberIDs[member.ID] = true

		if strings.TrimSpace(member.Path) == "" {
			return fmt.Errorf("%s: recipe member %q must have a path", m.Path, member.ID)
		}
		if err := validateManifestRelativeField(m, fmt.Sprintf("build.members[%q].path", member.ID), member.Path); err != nil {
			return err
		}
		switch member.Type {
		case "holon", "component":
		default:
			return fmt.Errorf("%s: recipe member %q type must be \"holon\" or \"component\"", m.Path, member.ID)
		}
		memberTypes[member.ID] = member.Type
	}

	if defaults := m.Manifest.Build.Defaults; defaults != nil && defaults.Target != "" {
		if _, ok := m.Manifest.Build.Targets[defaults.Target]; !ok {
			return fmt.Errorf("%s: recipe default target %q is not defined in build.targets", m.Path, defaults.Target)
		}
	}

	for targetName, target := range m.Manifest.Build.Targets {
		if len(target.Steps) == 0 {
			return fmt.Errorf("%s: target %q must declare at least one step", m.Path, targetName)
		}
		for i, step := range target.Steps {
			if step.actionCount() != 1 {
				return fmt.Errorf("%s: target %q step %d must declare exactly one action", m.Path, targetName, i+1)
			}
			if step.BuildMember != "" {
				if !memberIDs[step.BuildMember] {
					return fmt.Errorf("%s: target %q step %d references unknown member %q", m.Path, targetName, i+1, step.BuildMember)
				}
				if memberTypes[step.BuildMember] != "holon" {
					return fmt.Errorf("%s: target %q step %d build_member %q must reference a holon member", m.Path, targetName, i+1, step.BuildMember)
				}
			}
			if step.Exec != nil {
				if len(step.Exec.Argv) == 0 {
					return fmt.Errorf("%s: target %q step %d exec.argv must not be empty", m.Path, targetName, i+1)
				}
				if err := validateManifestRelativeField(m, fmt.Sprintf("build.targets[%q].steps[%d].exec.cwd", targetName, i+1), step.Exec.Cwd); err != nil {
					return err
				}
			}
			if step.Copy != nil {
				if err := validateManifestRelativeField(m, fmt.Sprintf("build.targets[%q].steps[%d].copy.from", targetName, i+1), step.Copy.From); err != nil {
					return err
				}
				if err := validateManifestRelativeField(m, fmt.Sprintf("build.targets[%q].steps[%d].copy.to", targetName, i+1), step.Copy.To); err != nil {
					return err
				}
			}
			if step.AssertFile != nil {
				if err := validateManifestRelativeField(m, fmt.Sprintf("build.targets[%q].steps[%d].assert_file.path", targetName, i+1), step.AssertFile.Path); err != nil {
					return err
				}
			}
			if step.CopyArtifact != nil {
				if !memberIDs[step.CopyArtifact.From] {
					return fmt.Errorf("%s: target %q step %d copy_artifact references unknown member %q", m.Path, targetName, i+1, step.CopyArtifact.From)
				}
				if memberTypes[step.CopyArtifact.From] != "holon" {
					return fmt.Errorf("%s: target %q step %d copy_artifact %q must reference a holon member", m.Path, targetName, i+1, step.CopyArtifact.From)
				}
				if err := validateManifestRelativeField(m, fmt.Sprintf("build.targets[%q].steps[%d].copy_artifact.to", targetName, i+1), step.CopyArtifact.To); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateList(field string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("%s cannot contain empty values", field)
		}
		if _, ok := seen[trimmed]; ok {
			return fmt.Errorf("%s contains duplicate value %q", field, trimmed)
		}
		seen[trimmed] = struct{}{}
	}
	return nil
}

func validateSequences(m *LoadedManifest) error {
	seenSequences := make(map[string]struct{}, len(m.Manifest.Sequences))
	for _, sequence := range m.Manifest.Sequences {
		name := strings.TrimSpace(sequence.Name)
		if name == "" {
			return fmt.Errorf("%s: sequence name must not be empty", m.Path)
		}
		if _, exists := seenSequences[name]; exists {
			return fmt.Errorf("%s: duplicate sequence %q", m.Path, name)
		}
		seenSequences[name] = struct{}{}

		if len(sequence.Steps) == 0 {
			return fmt.Errorf("%s: sequence %q must declare at least one step", m.Path, name)
		}
		for i, step := range sequence.Steps {
			if strings.TrimSpace(step) == "" {
				return fmt.Errorf("%s: sequence %q step %d must not be empty", m.Path, name, i+1)
			}
		}

		seenParams := make(map[string]struct{}, len(sequence.Params))
		for _, param := range sequence.Params {
			paramName := strings.TrimSpace(param.Name)
			if paramName == "" {
				return fmt.Errorf("%s: sequence %q parameter name must not be empty", m.Path, name)
			}
			if _, exists := seenParams[paramName]; exists {
				return fmt.Errorf("%s: sequence %q has duplicate parameter %q", m.Path, name, paramName)
			}
			seenParams[paramName] = struct{}{}
		}
	}
	return nil
}

func normalizeManifest(m *LoadedManifest) error {
	normalizedPlatforms := make([]string, 0, len(m.Manifest.Platforms))
	for _, platform := range m.Manifest.Platforms {
		normalizedPlatforms = append(normalizedPlatforms, normalizePlatformName(platform))
	}
	m.Manifest.Platforms = normalizedPlatforms

	if defaults := m.Manifest.Build.Defaults; defaults != nil && defaults.Target != "" {
		target, err := normalizeBuildTarget(defaults.Target)
		if err != nil {
			return fmt.Errorf("%s: build.defaults.target: %w", m.Path, err)
		}
		defaults.Target = target
	}
	if defaults := m.Manifest.Build.Defaults; defaults != nil && defaults.Mode != "" {
		defaults.Mode = normalizeBuildMode(defaults.Mode)
		if !isValidBuildMode(defaults.Mode) {
			return fmt.Errorf("%s: build.defaults.mode %q must be one of debug, release, profile", m.Path, defaults.Mode)
		}
	}

	if len(m.Manifest.Build.Targets) > 0 {
		normalizedTargets := make(map[string]RecipeTarget, len(m.Manifest.Build.Targets))
		for target, recipeTarget := range m.Manifest.Build.Targets {
			normalizedTarget, err := normalizeBuildTarget(target)
			if err != nil {
				return fmt.Errorf("%s: build.targets[%q]: %w", m.Path, target, err)
			}
			if _, exists := normalizedTargets[normalizedTarget]; exists {
				return fmt.Errorf("%s: duplicate recipe target after normalization: %q", m.Path, normalizedTarget)
			}
			normalizedTargets[normalizedTarget] = recipeTarget
		}
		m.Manifest.Build.Targets = normalizedTargets
	}

	return nil
}

func validateManifestRelativeField(m *LoadedManifest, field, relPath string) error {
	if _, err := resolveManifestPattern(m.Dir, relPath); err != nil {
		return fmt.Errorf("%s: %s: %w", m.Path, field, err)
	}
	return nil
}

func validateBinaryName(m *LoadedManifest, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%s: artifacts.binary must not be empty", m.Path)
	}
	if filepath.Base(trimmed) != trimmed || strings.Contains(trimmed, "/") || strings.Contains(trimmed, `\`) {
		return fmt.Errorf("%s: artifacts.binary must be a binary name, not a path", m.Path)
	}
	if trimmed == "." || trimmed == ".." {
		return fmt.Errorf("%s: artifacts.binary must be a binary name, not %q", m.Path, trimmed)
	}
	return nil
}

func resolveManifestPath(baseDir, rel string) (string, error) {
	fullPath, err := resolveManifestPattern(baseDir, rel)
	if err != nil {
		return "", err
	}
	if containsGlob(rel) {
		return "", fmt.Errorf("path must not contain glob patterns")
	}
	return fullPath, nil
}

func resolveManifestPattern(baseDir, rel string) (string, error) {
	trimmed := strings.TrimSpace(rel)
	if trimmed == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("path must be relative to the manifest file")
	}
	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	return filepath.Join(baseDir, cleaned), nil
}

func containsGlob(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func (s RecipeStep) actionCount() int {
	count := 0
	if strings.TrimSpace(s.BuildMember) != "" {
		count++
	}
	if s.Exec != nil {
		count++
	}
	if s.Copy != nil {
		count++
	}
	if s.AssertFile != nil {
		count++
	}
	if s.CopyArtifact != nil {
		count++
	}
	return count
}

func isValidPlatform(platform string) bool {
	switch normalizePlatformName(platform) {
	case "aix", "android", "ios", "ios-simulator", "js", "linux", "macos", "netbsd", "openbsd",
		"plan9", "solaris", "tvos", "tvos-simulator", "visionos", "visionos-simulator", "wasip1", "watchos", "watchos-simulator", "windows",
		"dragonfly", "freebsd", "illumos":
		return true
	default:
		return false
	}
}
