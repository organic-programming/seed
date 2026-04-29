package holons

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/progress"
	"github.com/organic-programming/grace-op/internal/sdkprebuilts"
)

type Operation string

const (
	OperationCheck Operation = "check"
	OperationBuild Operation = "build"
	OperationTest  Operation = "test"
	OperationClean Operation = "clean"

	buildModeDebug   = "debug"
	buildModeRelease = "release"
	buildModeProfile = "profile"
)

// BuildOptions captures CLI/build request overrides before manifest defaults are applied.
type BuildOptions struct {
	Target            string
	Mode              string
	Hardened          bool
	DryRun            bool
	NoSign            bool
	Bump              bool
	NoAutoInstall     bool
	Progress          progress.Reporter
	ResolveRoot       *string
	ResolveSpecifiers int
	ResolveTimeout    int
	composite         *compositeBuildExecution
}

// BuildContext is the canonical build request used by runners and artifact resolution.
type BuildContext struct {
	Target           string
	Mode             string
	Hardened         bool
	DryRun           bool
	NoSign           bool
	Bump             bool
	NoAutoInstall    bool
	Progress         progress.Reporter
	SDKPrebuiltPaths map[string]string
	composite        *compositeBuildExecution
}

var (
	sdkPrebuiltLocate              = sdkprebuilts.Locate
	sdkPrebuiltInstall             = sdkprebuilts.Install
	sdkPrebuiltBuild               = sdkprebuilts.Build
	sdkPrebuiltListAvailable       = sdkprebuilts.ListAvailable
	sdkPrebuiltLocalSourceTreeHash = sdkprebuilts.LocalSourceTreeSHA256
)

type Report struct {
	Operation   string   `json:"operation"`
	Target      string   `json:"target"`
	Holon       string   `json:"holon"`
	Dir         string   `json:"dir"`
	Manifest    string   `json:"manifest"`
	Runner      string   `json:"runner,omitempty"`
	Kind        string   `json:"kind,omitempty"`
	Binary      string   `json:"binary,omitempty"`
	BuildTarget string   `json:"build_target,omitempty"`
	BuildMode   string   `json:"build_mode,omitempty"`
	Artifact    string   `json:"artifact,omitempty"`
	Commands    []string `json:"commands,omitempty"`
	Notes       []string `json:"notes,omitempty"`
	Children    []Report `json:"children,omitempty"`
}

type compositeDependencyNode struct {
	key       string
	label     string
	dir       string
	manifest  *LoadedManifest
	dependsOn []string
}

type compositeBuildSession struct {
	nodes     map[string]*compositeDependencyNode
	completed map[string]Report
}

type compositeBuildExecution struct {
	session                *compositeBuildSession
	skipDependencyPrebuild bool
}

type runner interface {
	check(*LoadedManifest, BuildContext) error
	build(*LoadedManifest, BuildContext, *Report) error
	test(*LoadedManifest, BuildContext, *Report) error
	clean(*LoadedManifest, BuildContext, *Report) error
}

// ExecuteLifecycle runs a lifecycle operation on a holon.
func ExecuteLifecycle(op Operation, ref string, opts ...BuildOptions) (Report, error) {
	var bo BuildOptions
	if len(opts) > 0 {
		bo = opts[0]
	}
	reporter := bo.Progress
	if reporter == nil {
		reporter = progress.Silence()
	}

	resolveSpecifiers := bo.ResolveSpecifiers
	if resolveSpecifiers == 0 {
		resolveSpecifiers = sdkdiscover.ALL
	}

	target, err := ResolveTargetWithOptions(ref, bo.ResolveRoot, resolveSpecifiers, bo.ResolveTimeout)
	if err != nil {
		return Report{Operation: string(op), Target: normalizedTarget(ref)}, err
	}
	if target.ManifestErr != nil {
		return baseReport(op, target, BuildContext{}), target.ManifestErr
	}
	if target.Manifest == nil {
		return baseReport(op, target, BuildContext{}), fmt.Errorf("no %s found in %s", identity.ProtoManifestFileName, target.RelativePath)
	}

	ctx, err := resolveBuildContext(target.Manifest, bo)
	if err != nil {
		return baseReport(op, target, BuildContext{}), err
	}
	if op != OperationBuild {
		ctx.DryRun = false
	}

	if op == OperationBuild && progress.BuildReporterLabel(reporter) == "" {
		if writer := progress.WriterFromReporter(reporter); writer != nil {
			reporter = progress.NewBuildReporter(writer, buildProgressLabel(target))
		}
	}
	ctx.Progress = reporter

	report := baseReport(op, target, ctx)
	r, err := runnerFor(target.Manifest)
	if err != nil {
		return report, err
	}

	if op != OperationClean && op != OperationBuild {
		reporter.Step("checking manifest...")
		reporter.Step("validating prerequisites...")
		if err := preflight(target.Manifest, &ctx); err != nil {
			return report, err
		}
		if err := r.check(target.Manifest, ctx); err != nil {
			return report, err
		}
	}

	switch op {
	case OperationCheck:
		report.Notes = append(report.Notes, "manifest and prerequisites validated")
		return report, nil
	case OperationBuild:
		reporter.Step("checking manifest...")
		if !ctx.DryRun {
			if reason := buildContextChangeCleanReason(target.Manifest, ctx); reason != "" {
				reporter.Step(reason)
				report.Notes = append(report.Notes, reason)
				cleanCtx := ctx
				cleanCtx.DryRun = false
				cleanCtx.Progress = reporter.Child()
				if err := r.clean(target.Manifest, cleanCtx, &Report{}); err != nil {
					return report, fmt.Errorf("clean for build state change: %w", err)
				}
			}
		}

		if shouldPrebuildCompositeDependencies(target.Manifest, ctx) {
			session, childReports, prepErr := prebuildCompositeDependencies(target.Manifest, ctx)
			report.Children = append(report.Children, childReports...)
			if prepErr != nil {
				err = prepErr
				break
			}
			ctx.composite = &compositeBuildExecution{
				session:                session,
				skipDependencyPrebuild: true,
			}
			ctx.Progress = reporter
		}

		// Proto stage: stage, parse, and compile descriptors.
		var stage *protoStageResult
		if !ctx.DryRun {
			var stageErr error
			stage, stageErr = protoStage(target.Manifest, reporter)
			if stageErr != nil {
				return report, stageErr
			}
		}
		if codegenErr := runCodegen(target.Manifest, ctx, stage, reporter, &report); codegenErr != nil {
			err = codegenErr
			break
		}

		reporter.Step("validating prerequisites...")
		if err := preflight(target.Manifest, &ctx); err != nil {
			return report, err
		}
		if err := r.check(target.Manifest, ctx); err != nil {
			return report, err
		}

		// Opt-in patch bump: only when --bump is set. On failure the original
		// version is restored; on success it sticks. Under --dry-run the
		// intended bump is previewed as a report note without writing.
		if ctx.Bump {
			keepVersion, restoreVersion, patchErr := autoIncrementPatch(target.Manifest, reporter, &report, ctx.DryRun)
			if patchErr != nil {
				err = fmt.Errorf("version increment: %w", patchErr)
				break
			}
			defer func() {
				if err != nil {
					restoreVersion()
				} else {
					keepVersion()
				}
			}()
		}

		if !ctx.DryRun {
			restoreDescribeSource, describeErr := generateDescribeSource(target.Manifest, reporter)
			if describeErr != nil {
				err = fmt.Errorf("describe source: %w", describeErr)
				break
			}
			defer func() {
				if err != nil {
					restoreDescribeSource()
				}
			}()

			restoreServeSource, serveErr := generateServeSource(target.Manifest, reporter)
			if serveErr != nil {
				err = fmt.Errorf("serve source: %w", serveErr)
				break
			}
			defer func() {
				if err != nil {
					restoreServeSource()
				}
			}()
		}

		restoreFn, tmplErr := processTemplates(target.Manifest, ctx, reporter)
		if tmplErr != nil {
			err = fmt.Errorf("template processing: %w", tmplErr)
			break
		}
		defer restoreFn()

		if hookErr := executePipelineHooks(target.Manifest, ctx, &report, target.Manifest.Manifest.Build.BeforeCommands); hookErr != nil {
			err = fmt.Errorf("before commands: %w", hookErr)
			break
		}

		err = r.build(target.Manifest, ctx, &report)
		if err != nil {
			break
		}

		if hookErr := executePipelineHooks(target.Manifest, ctx, &report, target.Manifest.Manifest.Build.AfterCommands); hookErr != nil {
			err = fmt.Errorf("after commands: %w", hookErr)
			break
		}
		if err == nil && !ctx.DryRun && !isAggregateBuildTarget(ctx.Target) {
			err = persistBuildMetadata(target.Manifest, ctx)
		}
		if err == nil && !isAggregateBuildTarget(ctx.Target) {
			reporter.Step("verifying artifact...")
			err = resolveArtifact(target.Manifest, ctx, &report)
		}
		if err == nil && !ctx.DryRun {
			_ = syncSharedHolonPackageCache(target.Manifest)
		}
		if err == nil && !ctx.DryRun && !isAggregateBuildTarget(ctx.Target) {
			err = verifyArtifact(target.Manifest, ctx, &report)
		}
		if err == nil && ctx.DryRun {
			report.Notes = append(report.Notes, "dry run — no commands executed")
		}
	case OperationTest:
		err = r.test(target.Manifest, ctx, &report)
	case OperationClean:
		err = r.clean(target.Manifest, ctx, &report)
		if err == nil {
			err = cleanSharedHolonPackageCache(target.Manifest, &report)
		}
	default:
		err = fmt.Errorf("unsupported operation %q", op)
	}

	return report, err
}

func ResolveBuildContext(manifest *LoadedManifest, opts BuildOptions) (BuildContext, error) {
	return resolveBuildContext(manifest, opts)
}

func resolveArtifact(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	artifactPath := manifest.ArtifactPath(ctx)
	if artifactPath == "" {
		return fmt.Errorf("no artifact declared for target %q mode %q", ctx.Target, ctx.Mode)
	}
	report.Artifact = workspaceRelativePath(artifactPath)
	return nil
}

// verifyArtifact checks the primary artifact exists after build (success contract).
func verifyArtifact(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	artifactPath := manifest.ArtifactPath(ctx)
	if artifactPath == "" {
		return fmt.Errorf("no artifact declared for target %q mode %q", ctx.Target, ctx.Mode)
	}
	if _, err := os.Stat(artifactPath); err != nil {
		return fmt.Errorf("primary artifact missing after build: %s", workspaceRelativePath(artifactPath))
	}
	if shouldWriteHolonJSON(manifest) {
		if _, err := os.Stat(manifest.BinaryPath()); err != nil {
			return fmt.Errorf("binary missing after build: %s", workspaceRelativePath(manifest.BinaryPath()))
		}
		holonJSONPath := filepath.Join(artifactPath, ".holon.json")
		if _, err := os.Stat(holonJSONPath); err != nil {
			return fmt.Errorf(".holon.json missing after build: %s", workspaceRelativePath(holonJSONPath))
		}
	}
	report.Artifact = workspaceRelativePath(artifactPath)
	report.Notes = append(report.Notes, fmt.Sprintf("artifact: %s", report.Artifact))
	return nil
}

func baseReport(op Operation, target *Target, ctx BuildContext) Report {
	holonName := filepath.Base(target.Dir)
	if ref := strings.TrimSpace(target.Ref); ref != "" && ref != "." && !strings.ContainsAny(ref, `/\`) {
		holonName = ref
	}

	report := Report{
		Operation:   string(op),
		Target:      normalizedTarget(target.Ref),
		Holon:       holonName,
		Dir:         target.RelativePath,
		BuildTarget: ctx.Target,
		BuildMode:   ctx.Mode,
	}
	if target.Manifest != nil {
		report.Manifest = workspaceRelativePath(target.Manifest.Path)
		report.Runner = target.Manifest.Manifest.Build.Runner
		report.Kind = target.Manifest.Manifest.Kind
		if binaryPath := target.Manifest.BinaryPath(); binaryPath != "" {
			report.Binary = workspaceRelativePath(binaryPath)
		}
		if op == OperationBuild {
			if artifactPath := target.Manifest.ArtifactPath(ctx); artifactPath != "" {
				report.Artifact = workspaceRelativePath(artifactPath)
			}
		}
	}
	return report
}

func buildProgressLabel(target *Target) string {
	if target == nil {
		return ""
	}
	if target.Identity != nil {
		if slug := strings.TrimSpace(target.Identity.Slug()); slug != "" {
			return slug
		}
	}
	if target.Manifest != nil {
		if slug := manifestSlug(target.Manifest); slug != "" {
			return slug
		}
	}
	return filepath.Base(target.Dir)
}

func shouldPrebuildCompositeDependencies(manifest *LoadedManifest, ctx BuildContext) bool {
	if manifest == nil {
		return false
	}
	if ctx.composite != nil && ctx.composite.skipDependencyPrebuild {
		return false
	}
	return manifest.Manifest.Kind == KindComposite &&
		manifest.Manifest.Build.Runner == RunnerRecipe &&
		!isAggregateBuildTarget(ctx.Target)
}

func prebuildCompositeDependencies(manifest *LoadedManifest, ctx BuildContext) (*compositeBuildSession, []Report, error) {
	session := &compositeBuildSession{
		nodes:     make(map[string]*compositeDependencyNode),
		completed: make(map[string]Report),
	}

	order, err := resolveCompositeDependencyOrder(manifest, ctx, session)
	if err != nil {
		return session, nil, err
	}
	if len(order) == 0 {
		return session, nil, nil
	}

	writer := progress.WriterFromReporter(ctx.Progress)
	reports := make([]Report, 0, len(order))
	for _, node := range order {
		dependencyStart := time.Now()
		if !ctx.DryRun && dependencyIsFresh(node.manifest, ctx) {
			report := dependencyFreshReport(node, ctx)
			session.completed[node.key] = report
			reports = append(reports, report)
			if writer != nil {
				writer.FreezeAt(buildSuccessLine(node.label), dependencyStart)
			}
			continue
		}

		var depReporter progress.Reporter = progress.Silence()
		if writer != nil {
			depReporter = progress.NewBuildReporterWithStart(writer, node.label, dependencyStart)
		}

		childReport, buildErr := ExecuteLifecycle(OperationBuild, node.dir, BuildOptions{
			Target:    ctx.Target,
			Mode:      ctx.Mode,
			Hardened:  ctx.Hardened,
			DryRun:    ctx.DryRun,
			NoSign:    ctx.NoSign,
			Progress:  depReporter,
			composite: &compositeBuildExecution{session: session, skipDependencyPrebuild: true},
		})
		session.completed[node.key] = childReport
		reports = append(reports, childReport)
		if buildErr != nil {
			return session, reports, fmt.Errorf("dependency %q: %w", node.label, buildErr)
		}
		if writer != nil {
			writer.FreezeAt(buildSuccessLine(node.label), dependencyStart)
		}
	}

	return session, reports, nil
}

func resolveCompositeDependencyOrder(manifest *LoadedManifest, ctx BuildContext, session *compositeBuildSession) ([]*compositeDependencyNode, error) {
	if manifest == nil {
		return nil, nil
	}
	if session == nil {
		session = &compositeBuildSession{
			nodes:     make(map[string]*compositeDependencyNode),
			completed: make(map[string]Report),
		}
	}

	order := make([]*compositeDependencyNode, 0)
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	stack := make([]string, 0)

	var visit func(*compositeDependencyNode) error
	visit = func(node *compositeDependencyNode) error {
		if node == nil {
			return nil
		}
		if visited[node.key] {
			return nil
		}
		if visiting[node.key] {
			cycle := append(append([]string(nil), stack...), node.label)
			return fmt.Errorf("composite dependency cycle: %s", strings.Join(cycle, " -> "))
		}

		visiting[node.key] = true
		stack = append(stack, node.label)
		for _, depKey := range node.dependsOn {
			if err := visit(session.nodes[depKey]); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		visiting[node.key] = false
		visited[node.key] = true
		order = append(order, node)
		return nil
	}

	directMembers, err := compositeHolonMembers(manifest, ctx, session)
	if err != nil {
		return nil, err
	}
	for _, node := range directMembers {
		if err := visit(node); err != nil {
			return nil, err
		}
	}

	return order, nil
}

func compositeHolonMembers(manifest *LoadedManifest, ctx BuildContext, session *compositeBuildSession) ([]*compositeDependencyNode, error) {
	nodes := make([]*compositeDependencyNode, 0)
	excludedMembers := resolveHardenedExcludedMembers(manifest, ctx)
	for _, member := range manifest.Manifest.Build.Members {
		if member.Type != "holon" {
			continue
		}
		if _, excluded := excludedMembers[member.ID]; excluded {
			continue
		}
		memberDir, err := manifest.ResolveManifestPath(member.Path)
		if err != nil {
			return nil, fmt.Errorf("recipe member %q path: %w", member.ID, err)
		}
		node, err := compositeNodeForDir(memberDir, ctx, session)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func compositeNodeForDir(dir string, ctx BuildContext, session *compositeBuildSession) (*compositeDependencyNode, error) {
	memberManifest, err := LoadManifest(dir)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(memberManifest.Dir); resolveErr == nil {
		memberManifest.Dir = resolved
	}

	key := compositeDependencyKey(memberManifest)
	if existing, ok := session.nodes[key]; ok {
		return existing, nil
	}

	node := &compositeDependencyNode{
		key:      key,
		label:    manifestSlug(memberManifest),
		dir:      memberManifest.Dir,
		manifest: memberManifest,
	}
	session.nodes[key] = node

	dependencies, err := compositeHolonMembers(memberManifest, ctx, session)
	if err != nil {
		return nil, err
	}
	node.dependsOn = make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		node.dependsOn = append(node.dependsOn, dependency.key)
	}

	return node, nil
}

func compositeDependencyKey(manifest *LoadedManifest) string {
	if manifest == nil {
		return ""
	}
	if uuid := strings.TrimSpace(manifest.Manifest.UUID); uuid != "" {
		return uuid
	}
	resolvedDir, err := filepath.EvalSymlinks(manifest.Dir)
	if err == nil {
		return resolvedDir
	}
	return filepath.Clean(manifest.Dir)
}

func manifestSlug(manifest *LoadedManifest) string {
	if manifest == nil {
		return ""
	}
	if binary := strings.TrimSpace(manifest.BinaryName()); binary != "" {
		return binary
	}
	if manifest.Name != "" {
		return strings.TrimSpace(manifest.Name)
	}
	name := strings.ToLower(strings.TrimSpace(manifest.Manifest.GivenName + "-" + strings.TrimSuffix(manifest.Manifest.FamilyName, "?")))
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.Trim(name, "-")
	if name != "" {
		return name
	}
	return filepath.Base(manifest.Dir)
}

func resolveHardenedExcludedMembers(manifest *LoadedManifest, ctx BuildContext) map[string]string {
	if manifest == nil || !ctx.Hardened {
		return nil
	}

	excluded := make(map[string]string)
	for _, member := range manifest.Manifest.Build.Members {
		if member.Type != "holon" {
			continue
		}
		memberDir, err := manifest.ResolveManifestPath(member.Path)
		if err != nil {
			continue
		}
		memberManifest, loadErr := LoadManifest(memberDir)
		if loadErr != nil {
			continue
		}
		runnerName := strings.TrimSpace(memberManifest.Manifest.Build.Runner)
		if runnerProducesStandaloneArtifact(runnerName) {
			continue
		}
		excluded[member.ID] = runnerName
		if ctx.Progress != nil {
			ctx.Progress.Step(fmt.Sprintf("hardened: skipping %s (runner %q not standalone)", member.ID, runnerName))
		}
	}
	return excluded
}

func recipeExcludedStepReason(step RecipeStep, excludedMembers map[string]string) string {
	if len(excludedMembers) == 0 {
		return ""
	}
	if step.BuildMember != "" {
		if runnerName, excluded := excludedMembers[step.BuildMember]; excluded {
			return fmt.Sprintf(`hardened: skipped build_member %q (runner %q not standalone)`, step.BuildMember, runnerName)
		}
	}
	if step.CopyArtifact != nil {
		if runnerName, excluded := excludedMembers[step.CopyArtifact.From]; excluded {
			return fmt.Sprintf(`hardened: skipped copy_artifact from %q (runner %q not standalone)`, step.CopyArtifact.From, runnerName)
		}
	}
	return ""
}

func dependencyFreshReport(node *compositeDependencyNode, ctx BuildContext) Report {
	target := &Target{
		Ref:          node.label,
		Dir:          node.dir,
		RelativePath: workspaceRelativePath(node.dir),
		Manifest:     node.manifest,
	}
	report := baseReport(OperationBuild, target, ctx)
	report.Notes = append(report.Notes, "fresh dependency — skipped rebuild")
	return report
}

func dependencyIsFresh(manifest *LoadedManifest, ctx BuildContext) bool {
	if manifest == nil {
		return false
	}
	if reason := buildContextChangeCleanReason(manifest, ctx); reason != "" {
		return false
	}
	sourceModTime, err := latestSourceTreeModTime(manifest.Dir)
	if err != nil {
		return false
	}

	for _, markerPath := range []string{
		sharedHolonPackageMarker(manifest),
		buildStatePath(manifest),
		buildFreshnessMarker(manifest, ctx),
	} {
		if markerPath == "" {
			continue
		}
		markerInfo, statErr := os.Stat(markerPath)
		if statErr != nil {
			continue
		}
		if !markerInfo.ModTime().Before(sourceModTime) {
			return true
		}
	}

	return false
}

func buildFreshnessMarker(manifest *LoadedManifest, ctx BuildContext) string {
	if manifest == nil {
		return ""
	}
	if binaryPath := manifest.BinaryPath(); binaryPath != "" {
		if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
			return binaryPath
		}
	}
	holonJSONPath := filepath.Join(manifest.HolonPackageDir(), ".holon.json")
	if info, err := os.Stat(holonJSONPath); err == nil && !info.IsDir() {
		return holonJSONPath
	}
	artifactPath := manifest.ArtifactPath(ctx)
	if artifactPath == "" {
		return ""
	}
	if info, err := os.Stat(artifactPath); err == nil {
		if info.IsDir() {
			return filepath.Join(artifactPath, ".holon.json")
		}
		return artifactPath
	}
	return ""
}

func syncSharedHolonPackageCache(manifest *LoadedManifest) error {
	if manifest == nil {
		return nil
	}
	srcDir := manifest.HolonPackageDir()
	if info, err := os.Stat(srcDir); err != nil || !info.IsDir() {
		return nil
	}
	return copyArtifact(srcDir, sharedHolonPackageDir(manifest))
}

func cleanSharedHolonPackageCache(manifest *LoadedManifest, report *Report) error {
	if manifest == nil {
		return nil
	}
	cacheDir := sharedHolonPackageDir(manifest)
	if info, err := os.Stat(cacheDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	} else if !info.IsDir() {
		return os.Remove(cacheDir)
	}
	if err := os.RemoveAll(cacheDir); err != nil {
		return err
	}
	if report != nil {
		report.Notes = append(report.Notes, "removed shared holon package cache")
	}
	return nil
}

func sharedHolonPackageDir(manifest *LoadedManifest) string {
	base := filepath.Join(os.TempDir(), "grace-op-holon-cache")
	name := "holon"
	key := "."
	if manifest != nil {
		if binary := strings.TrimSpace(manifest.BinaryName()); binary != "" {
			name = binary
		} else if manifest.Name != "" {
			name = strings.TrimSpace(manifest.Name)
		}
		if dir := strings.TrimSpace(manifest.Dir); dir != "" {
			key = filepath.Clean(dir)
		}
	}

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(key))
	return filepath.Join(base, fmt.Sprintf("%s-%x.holon", name, hasher.Sum64()))
}

func sharedHolonPackageMarker(manifest *LoadedManifest) string {
	cachedDir := sharedHolonPackageDir(manifest)
	holonJSONPath := filepath.Join(cachedDir, ".holon.json")
	if info, err := os.Stat(holonJSONPath); err == nil && !info.IsDir() {
		return holonJSONPath
	}
	if binary := strings.TrimSpace(manifest.BinaryName()); binary != "" {
		cachedBinary := filepath.Join(cachedDir, "bin", runtimeArchitecture(), hostExecutableName(binary))
		if info, err := os.Stat(cachedBinary); err == nil && !info.IsDir() {
			return cachedBinary
		}
	}
	if info, err := os.Stat(cachedDir); err == nil && info.IsDir() {
		return cachedDir
	}
	return ""
}

func availableHolonPackageDir(manifest *LoadedManifest) string {
	localDir := manifest.HolonPackageDir()
	if info, err := os.Stat(localDir); err == nil && info.IsDir() {
		return localDir
	}
	cachedDir := sharedHolonPackageDir(manifest)
	if info, err := os.Stat(cachedDir); err == nil && info.IsDir() {
		return cachedDir
	}
	return localDir
}

func latestSourceTreeModTime(root string) (time.Time, error) {
	var newest time.Time
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipSourceDir(filepath.Base(path), path, root) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}
	return newest, nil
}

func shouldSkipSourceDir(name, path, root string) bool {
	if filepath.Clean(path) == filepath.Clean(root) {
		return false
	}
	switch name {
	case ".git", ".op", "node_modules", "vendor", "build", "testdata":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func buildSuccessLine(label string) string {
	return fmt.Sprintf("built %s… ✓", strings.TrimSpace(label))
}

func cleanSuccessLine(label string, elapsed time.Duration) string {
	return fmt.Sprintf("✓ cleaned %s in %s", strings.TrimSpace(label), progress.FormatElapsed(elapsed))
}

func preflight(manifest *LoadedManifest, ctx *BuildContext) error {
	if ctx == nil {
		return fmt.Errorf("build context required")
	}
	if !isAggregateBuildTarget(ctx.Target) && !manifest.SupportsTarget(ctx.Target) {
		return fmt.Errorf("target %q is not supported by %s", ctx.Target, workspaceRelativePath(manifest.Path))
	}

	for _, requiredFile := range manifest.Manifest.Requires.Files {
		fullPath, err := resolveManifestPattern(manifest.Dir, requiredFile)
		if err != nil {
			return fmt.Errorf("invalid required file %q: %w", requiredFile, err)
		}
		if containsGlob(requiredFile) {
			matches, globErr := filepath.Glob(fullPath)
			if globErr != nil {
				return fmt.Errorf("invalid required file %q: %w", requiredFile, globErr)
			}
			if len(matches) == 0 {
				return fmt.Errorf("missing required file %q (%s)", requiredFile, workspaceRelativePath(fullPath))
			}
			continue
		}
		if _, err := os.Stat(fullPath); err != nil {
			return fmt.Errorf("missing required file %q (%s)", requiredFile, workspaceRelativePath(fullPath))
		}
	}

	for _, command := range requiredCommands(manifest) {
		if _, err := exec.LookPath(command); err != nil {
			return fmt.Errorf("missing required command %q on PATH; %s", command, installHint(command))
		}
	}

	if err := resolveRequiredSDKPrebuilts(manifest, ctx); err != nil {
		return err
	}
	if err := validateCodegenPlugins(manifest); err != nil {
		return err
	}

	return nil
}

func resolveRequiredSDKPrebuilts(manifest *LoadedManifest, ctx *BuildContext) error {
	if manifest == nil || len(manifest.Manifest.Requires.SDKPrebuilts) == 0 {
		return nil
	}
	hostTarget, err := sdkprebuilts.HostTriplet()
	if err != nil {
		return fmt.Errorf("resolve SDK prebuilt host target: %w", err)
	}

	paths := make(map[string]string, len(ctx.SDKPrebuiltPaths)+len(manifest.Manifest.Requires.SDKPrebuilts))
	for lang, path := range ctx.SDKPrebuiltPaths {
		paths[lang] = path
	}
	for _, required := range manifest.Manifest.Requires.SDKPrebuilts {
		lang, err := sdkprebuilts.NormalizeLang(required)
		if err != nil {
			return fmt.Errorf("requires.sdk_prebuilts: %w", err)
		}
		// Explicit env-var override (OP_SDK_<LANG>_PATH) wins over auto-install:
		// it expresses an explicit dev-time choice to use a path the user
		// controls, bypassing any locate/install/build logic.
		envPath, envErr := explicitSDKPrebuiltPath(lang)
		if envErr != nil {
			return envErr
		}
		if envPath != "" {
			paths[lang] = envPath
			continue
		}
		prebuilt, err := resolveRequiredSDKPrebuilt(context.Background(), lang, hostTarget, ctx)
		if err != nil {
			return err
		}
		paths[lang] = prebuilt.Path
	}
	ctx.SDKPrebuiltPaths = paths
	return nil
}

func explicitSDKPrebuiltPath(lang string) (string, error) {
	envName := sdkPrebuiltEnvName(lang)
	path := strings.TrimSpace(os.Getenv(envName))
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		// Env var set but path is invalid: ignore silently and fall through to
		// the standard locate/auto-install path. The user may have a stale
		// override from a previous run; we don't want to fail loudly when a
		// valid installed prebuilt is otherwise reachable.
		return "", nil
	}
	if !info.IsDir() {
		return "", nil
	}
	return path, nil
}

// ResolveRequiredSDKPrebuilts applies the SDK prebuilt preflight resolver for
// commands such as inspect that need SDK availability without running the full
// lifecycle preflight.
func ResolveRequiredSDKPrebuilts(manifest *LoadedManifest, ctx *BuildContext) error {
	return resolveRequiredSDKPrebuilts(manifest, ctx)
}

const (
	sdkPrebuiltActionSkip    = "skip"
	sdkPrebuiltActionInstall = "install"
	sdkPrebuiltActionBuild   = "build"
)

func resolveRequiredSDKPrebuilt(ctx context.Context, lang, hostTarget string, buildCtx *BuildContext) (sdkprebuilts.Prebuilt, error) {
	installed, installedErr := sdkPrebuiltLocate(sdkprebuilts.QueryOptions{Lang: lang, Target: hostTarget})
	if buildCtx.NoAutoInstall {
		if installedErr == nil {
			return installed, nil
		}
		return sdkprebuilts.Prebuilt{}, missingSDKPrebuiltError(lang, hostTarget)
	}

	sourceHash, sourceExists, sourceErr := sdkPrebuiltLocalSourceTreeHash(lang)
	if sourceErr != nil {
		return sdkprebuilts.Prebuilt{}, fmt.Errorf("hash local SDK source for %s: %w", lang, sourceErr)
	}
	if installedErr == nil && installedSDKPrebuiltSatisfiesSource(installed, sourceHash, sourceExists) {
		return installed, nil
	}

	unlock, err := lockSDKPrebuiltInstall(lang)
	if err != nil {
		return sdkprebuilts.Prebuilt{}, err
	}
	defer unlock()

	installed, installedErr = sdkPrebuiltLocate(sdkprebuilts.QueryOptions{Lang: lang, Target: hostTarget})
	sourceHash, sourceExists, sourceErr = sdkPrebuiltLocalSourceTreeHash(lang)
	if sourceErr != nil {
		return sdkprebuilts.Prebuilt{}, fmt.Errorf("hash local SDK source for %s: %w", lang, sourceErr)
	}
	if installedErr == nil && installedSDKPrebuiltSatisfiesSource(installed, sourceHash, sourceExists) {
		return installed, nil
	}

	action, version, detail, err := decideSDKPrebuiltAutoResolution(lang, hostTarget, installed, installedErr == nil, sourceHash, sourceExists)
	if err != nil {
		return sdkprebuilts.Prebuilt{}, err
	}
	if action == sdkPrebuiltActionSkip && installedErr == nil {
		return installed, nil
	}

	started := time.Now()
	reporter := buildCtx.Progress
	if reporter == nil {
		reporter = progress.Silence()
	}
	switch action {
	case sdkPrebuiltActionInstall:
		reporter.Step(fmt.Sprintf("auto-installing SDK prebuilt %q v%s...", lang, version))
		prebuilt, _, err := sdkPrebuiltInstall(ctx, sdkprebuilts.InstallOptions{
			Lang:    lang,
			Target:  hostTarget,
			Version: version,
		})
		if err != nil {
			return sdkprebuilts.Prebuilt{}, err
		}
		reporter.Step(fmt.Sprintf("installed in %s", progress.FormatElapsed(time.Since(started))))
		reporter.Step(fmt.Sprintf("auto-resolved %s via install (%s)", lang, detail))
		return prebuilt, nil
	case sdkPrebuiltActionBuild:
		reporter.Step(fmt.Sprintf("auto-building SDK prebuilt %q v%s...", lang, version))
		prebuilt, _, err := sdkPrebuiltBuild(ctx, sdkprebuilts.BuildOptions{
			Lang:              lang,
			Target:            hostTarget,
			Version:           version,
			Force:             true,
			InstallAfterBuild: true,
		})
		if err != nil {
			return sdkprebuilts.Prebuilt{}, err
		}
		reporter.Step(fmt.Sprintf("installed in %s", progress.FormatElapsed(time.Since(started))))
		reporter.Step(fmt.Sprintf("auto-resolved %s via build (%s)", lang, detail))
		return prebuilt, nil
	default:
		return sdkprebuilts.Prebuilt{}, missingSDKPrebuiltError(lang, hostTarget)
	}
}

func decideSDKPrebuiltAutoResolution(lang, hostTarget string, installed sdkprebuilts.Prebuilt, installedOK bool, sourceHash string, sourceExists bool) (string, string, string, error) {
	version, _ := sdkprebuilts.DefaultVersion(lang)
	releases, releaseErr := releasedSDKPrebuilts(lang, hostTarget)
	if !sourceExists {
		if releaseErr == nil && len(releases) > 0 {
			version = releases[0].Version
		}
		return sdkPrebuiltActionInstall, version, fmt.Sprintf("download v%s", version), nil
	}

	if releaseErr == nil {
		for _, release := range releases {
			if strings.TrimSpace(release.SourceTreeSHA256) == "" ||
				!strings.EqualFold(strings.TrimSpace(release.SourceTreeSHA256), strings.TrimSpace(sourceHash)) {
				continue
			}
			if installedOK && strings.TrimSpace(installed.Version) == strings.TrimSpace(release.Version) {
				return sdkPrebuiltActionSkip, release.Version, "", nil
			}
			return sdkPrebuiltActionInstall, release.Version, fmt.Sprintf("download v%s", release.Version), nil
		}
	}

	detail := "source diverges from published releases"
	if releaseErr == nil && len(releases) > 0 {
		version = releases[0].Version
		detail = fmt.Sprintf("source diverges from published v%s", releases[0].Version)
	}
	return sdkPrebuiltActionBuild, version, detail, nil
}

func installedSDKPrebuiltSatisfiesSource(prebuilt sdkprebuilts.Prebuilt, sourceHash string, sourceExists bool) bool {
	if !sourceExists {
		return true
	}
	return strings.TrimSpace(prebuilt.SourceTreeSHA256) != "" &&
		strings.EqualFold(strings.TrimSpace(prebuilt.SourceTreeSHA256), strings.TrimSpace(sourceHash))
}

func releasedSDKPrebuilts(lang, hostTarget string) ([]sdkprebuilts.Prebuilt, error) {
	available, _, err := sdkPrebuiltListAvailable(lang)
	if err != nil {
		return nil, err
	}
	releases := make([]sdkprebuilts.Prebuilt, 0, len(available))
	for _, prebuilt := range available {
		if strings.TrimSpace(prebuilt.Target) == hostTarget {
			releases = append(releases, prebuilt)
		}
	}
	return releases, nil
}

func lockSDKPrebuiltInstall(lang string) (func(), error) {
	dir := filepath.Join(sdkprebuilts.SDKRoot(), lang)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create SDK prebuilt lock dir %s: %w", dir, err)
	}
	f, err := lockFileExclusive(filepath.Join(dir, ".install.lock"))
	if err != nil {
		return nil, err
	}
	return func() {
		_ = unlockFileExclusive(f)
	}, nil
}

func missingSDKPrebuiltError(lang, hostTarget string) error {
	return fmt.Errorf("missing SDK prebuilt %q for host target %s; run `op sdk install %s` to install prebuilt native libraries (~30 sec download, no source compilation)", lang, hostTarget, lang)
}

func sdkPrebuiltEnv(ctx BuildContext) []string {
	if len(ctx.SDKPrebuiltPaths) == 0 {
		return nil
	}
	langs := make([]string, 0, len(ctx.SDKPrebuiltPaths))
	for lang := range ctx.SDKPrebuiltPaths {
		langs = append(langs, lang)
	}
	sort.Strings(langs)

	env := make([]string, 0, len(langs))
	for _, lang := range langs {
		env = append(env, sdkPrebuiltEnvName(lang)+"="+ctx.SDKPrebuiltPaths[lang])
	}
	return env
}

func sdkPrebuiltEnvName(lang string) string {
	return "OP_SDK_" + strings.ToUpper(strings.TrimSpace(lang)) + "_PATH"
}

// templateData is the data available to Go template expressions in build templates.
type templateData struct {
	Version    string
	UUID       string
	GivenName  string
	FamilyName string
	Motto      string
	Composer   string
	Status     string
	Born       string
	Hardened   bool
}

// processTemplates resolves Go template expressions in declared build template files.
// Returns a restore function that writes back the original bytes (always call via defer).
func processTemplates(manifest *LoadedManifest, ctx BuildContext, reporter progress.Reporter) (restore func(), err error) {
	templates := manifest.Manifest.Build.Templates
	if len(templates) == 0 {
		return func() {}, nil
	}

	data := templateData{
		Version:    manifest.Manifest.Version,
		UUID:       manifest.Manifest.UUID,
		GivenName:  manifest.Manifest.GivenName,
		FamilyName: manifest.Manifest.FamilyName,
		Motto:      manifest.Manifest.Motto,
		Composer:   manifest.Manifest.Composer,
		Status:     manifest.Manifest.Status,
		Born:       manifest.Manifest.Born,
		Hardened:   ctx.Hardened,
	}

	// Read originals into memory and write resolved content.
	type savedFile struct {
		path     string
		original []byte
		mode     os.FileMode
	}
	var saved []savedFile

	restore = func() {
		for _, sf := range saved {
			_ = os.WriteFile(sf.path, sf.original, sf.mode)
		}
	}

	for _, relPath := range templates {
		absPath, resolveErr := manifest.ResolveManifestPath(relPath)
		if resolveErr != nil {
			restore()
			return func() {}, fmt.Errorf("template %q: %w", relPath, resolveErr)
		}

		info, statErr := os.Stat(absPath)
		if statErr != nil {
			restore()
			return func() {}, fmt.Errorf("template %q not found: %w", relPath, statErr)
		}

		original, readErr := os.ReadFile(absPath)
		if readErr != nil {
			restore()
			return func() {}, fmt.Errorf("template %q: %w", relPath, readErr)
		}

		saved = append(saved, savedFile{path: absPath, original: original, mode: info.Mode()})

		rendered, renderErr := renderBuildTemplate(relPath, original, data)
		if renderErr != nil {
			restore()
			return func() {}, renderErr
		}

		if writeErr := os.WriteFile(absPath, rendered, info.Mode()); writeErr != nil {
			restore()
			return func() {}, fmt.Errorf("template %q write: %w", relPath, writeErr)
		}

		reporter.Step(fmt.Sprintf("template: %s", relPath))
	}

	return restore, nil
}

var versionLineRe = regexp.MustCompile(`(version:\s*")([0-9]+\.[0-9]+\.[0-9]+)(")`)
var semverLiteralRe = regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+\b`)

func renderBuildTemplate(relPath string, original []byte, data templateData) ([]byte, error) {
	if bytes.Contains(original, []byte("{{")) {
		tmpl, parseErr := template.New(relPath).Parse(string(original))
		if parseErr != nil {
			return nil, fmt.Errorf("template %q parse: %w", relPath, parseErr)
		}

		var buf bytes.Buffer
		if execErr := tmpl.Execute(&buf, data); execErr != nil {
			return nil, fmt.Errorf("template %q execute: %w", relPath, execErr)
		}
		return buf.Bytes(), nil
	}

	return []byte(semverLiteralRe.ReplaceAllString(string(original), data.Version)), nil
}

// autoIncrementPatch bumps the patch component of identity.version in the proto
// file and updates the in-memory manifest. When previewOnly is true, it
// computes the next version and records a preview note on the report without
// touching disk or the in-memory manifest. Returns two closures:
//   - keep: no-op (the bumped proto stays as-is)
//   - restore: writes back the original proto bytes
func autoIncrementPatch(manifest *LoadedManifest, reporter progress.Reporter, report *Report, previewOnly bool) (keep, restore func(), err error) {
	oldVersion := strings.TrimSpace(manifest.Manifest.Version)
	parts := strings.SplitN(oldVersion, ".", 3)
	if len(parts) != 3 {
		// No valid semver — skip bump silently.
		return func() {}, func() {}, nil
	}

	patch, convErr := strconv.Atoi(parts[2])
	if convErr != nil {
		return func() {}, func() {}, nil
	}

	newVersion := parts[0] + "." + parts[1] + "." + strconv.Itoa(patch+1)

	if previewOnly {
		note := fmt.Sprintf("would bump version: %s → %s", oldVersion, newVersion)
		reporter.Step(note)
		if report != nil {
			report.Notes = append(report.Notes, note)
		}
		return func() {}, func() {}, nil
	}

	protoPath := manifest.Path
	original, readErr := os.ReadFile(protoPath)
	if readErr != nil {
		return nil, nil, fmt.Errorf("read proto: %w", readErr)
	}

	info, statErr := os.Stat(protoPath)
	if statErr != nil {
		return nil, nil, fmt.Errorf("stat proto: %w", statErr)
	}

	// Replace the first occurrence of the version in the proto text.
	updated := versionLineRe.ReplaceAllFunc(original, func(match []byte) []byte {
		subs := versionLineRe.FindSubmatch(match)
		if string(subs[2]) == oldVersion {
			return append(append(subs[1], []byte(newVersion)...), subs[3]...)
		}
		return match
	})

	if err := os.WriteFile(protoPath, updated, info.Mode()); err != nil {
		return nil, nil, fmt.Errorf("write proto: %w", err)
	}

	// Update in-memory manifest so templates pick up the new version.
	manifest.Manifest.Version = newVersion

	reporter.Step(fmt.Sprintf("version: %s → %s", oldVersion, newVersion))

	restore = func() {
		_ = os.WriteFile(protoPath, original, info.Mode())
		manifest.Manifest.Version = oldVersion
	}
	keep = func() {} // new version stays in the proto

	return keep, restore, nil
}

func requiredCommands(manifest *LoadedManifest) []string {
	commands := append([]string{}, manifest.Manifest.Requires.Commands...)
	if manifest.Manifest.Kind == KindWrapper {
		commands = append(commands, manifest.Manifest.Delegates.Commands...)
	}
	return uniqueNonEmpty(commands)
}

func resolveBuildContext(manifest *LoadedManifest, opts BuildOptions) (BuildContext, error) {
	target := strings.TrimSpace(opts.Target)
	if target != "" {
		if strings.EqualFold(target, "darwin") {
			return BuildContext{}, fmt.Errorf("unsupported target %q (supported: macos, linux, windows, ios, ios-simulator, tvos, tvos-simulator, watchos, watchos-simulator, visionos, visionos-simulator, android, all)", target)
		}
		normalizedTarget, err := normalizeBuildTarget(target)
		if err != nil {
			return BuildContext{}, err
		}
		target = normalizedTarget
	} else if defaults := manifest.Manifest.Build.Defaults; defaults != nil && strings.TrimSpace(defaults.Target) != "" {
		target = defaults.Target
	} else {
		target = canonicalRuntimeTarget()
	}

	if isAggregateBuildTarget(target) && manifest.Manifest.Build.Runner != RunnerRecipe {
		return BuildContext{}, fmt.Errorf("target %q is only supported for recipe runners", target)
	}

	mode := strings.TrimSpace(opts.Mode)
	if mode != "" {
		mode = normalizeBuildMode(mode)
		if !isValidBuildMode(mode) {
			return BuildContext{}, fmt.Errorf("unsupported mode %q (supported: debug, release, profile)", opts.Mode)
		}
	} else if defaults := manifest.Manifest.Build.Defaults; defaults != nil && strings.TrimSpace(defaults.Mode) != "" {
		mode = normalizeBuildMode(defaults.Mode)
		if !isValidBuildMode(mode) {
			return BuildContext{}, fmt.Errorf("unsupported mode %q (supported: debug, release, profile)", defaults.Mode)
		}
	} else {
		mode = buildModeDebug
	}

	return BuildContext{
		Target:        target,
		Mode:          mode,
		Hardened:      opts.Hardened,
		DryRun:        opts.DryRun,
		NoSign:        opts.NoSign,
		Bump:          opts.Bump,
		NoAutoInstall: opts.NoAutoInstall,
		Progress:      opts.Progress,
		composite:     opts.composite,
	}, nil
}

func runnerFor(manifest *LoadedManifest) (runner, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest required")
	}
	name := strings.TrimSpace(manifest.Manifest.Build.Runner)
	r, ok := runnerRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unsupported runner %q", name)
	}
	return r, nil
}

type goModuleRunner struct{}

func (goModuleRunner) check(manifest *LoadedManifest, _ BuildContext) error {
	mainPackage := manifest.GoMainPackage()
	if strings.HasPrefix(mainPackage, ".") {
		fullPath, err := manifest.ResolveManifestPath(mainPackage)
		if err != nil {
			return fmt.Errorf("go main package %q: %w", mainPackage, err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return fmt.Errorf("go main package %q not found (%s)", mainPackage, workspaceRelativePath(fullPath))
		}
		if !info.IsDir() {
			return fmt.Errorf("go main package %q must be a directory", mainPackage)
		}
	}
	return nil
}

func (goModuleRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerGoModule, ctx); err != nil {
		return err
	}

	binaryPath := manifest.BinaryPath()
	args := []string{"go", "build", "-o", binaryPath, manifest.GoMainPackage()}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if ctx.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
		return err
	}
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}

	report.Notes = append(report.Notes, "binary built")
	return nil
}

func (goModuleRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerGoModule, ctx); err != nil {
		return err
	}
	args := []string{"go", "test", "./..."}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}

	report.Notes = append(report.Notes, "tests passed")
	return nil
}

func (goModuleRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

type cmakeRunner struct{}

func (cmakeRunner) check(manifest *LoadedManifest, _ BuildContext) error {
	return nil
}

func (r cmakeRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerCMake, ctx); err != nil {
		return err
	}

	config := cmakeBuildConfig(ctx.Mode)
	configureArgs := []string{
		"cmake",
		"-S", ".",
		"-B", manifest.CMakeBuildDir(),
		"-DCMAKE_BUILD_TYPE=" + config,
		"-DOP_PACKAGE_ARCH=" + runtimeArchitecture(),
	}
	buildArgs := []string{"cmake", "--build", manifest.CMakeBuildDir(), "--config", config}
	installMode := cmakeProjectUsesInstallRules(manifest.Dir)
	var installArgs []string
	if installMode {
		installPrefix := manifest.ArtifactPath(ctx)
		if installPrefix == "" {
			return fmt.Errorf("cmake install requires an artifact path")
		}
		configureArgs = append(configureArgs,
			"-DCMAKE_INSTALL_PREFIX="+installPrefix,
			"-DOP_USE_INSTALL_LAYOUT=ON",
		)
		installArgs = []string{"cmake", "--install", manifest.CMakeBuildDir(), "--config", config}
		report.Commands = append(report.Commands, commandString(configureArgs), commandString(buildArgs), commandString(installArgs))
	} else {
		binDir := filepath.Dir(manifest.BinaryPath())
		configureArgs = append(configureArgs,
			"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY="+binDir,
			"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY_DEBUG="+binDir,
			"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY_RELEASE="+binDir,
			"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY_RELWITHDEBINFO="+binDir,
			"-DOP_USE_INSTALL_LAYOUT=OFF",
		)
		report.Commands = append(report.Commands, commandString(configureArgs), commandString(buildArgs))
	}
	if ctx.DryRun {
		return nil
	}
	if err := os.MkdirAll(manifest.CMakeBuildDir(), 0755); err != nil {
		return err
	}
	ctx.Progress.Step(commandString(configureArgs))
	if output, err := runCommandWithEnv(manifest.Dir, configureArgs, sdkPrebuiltEnv(ctx)); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	ctx.Progress.Step(commandString(buildArgs))
	if output, err := runCommandWithEnv(manifest.Dir, buildArgs, sdkPrebuiltEnv(ctx)); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	if installMode {
		installPrefix := manifest.ArtifactPath(ctx)
		if err := os.RemoveAll(installPrefix); err != nil {
			return fmt.Errorf("clear install prefix: %w", err)
		}
		if err := os.MkdirAll(installPrefix, 0o755); err != nil {
			return fmt.Errorf("create install prefix: %w", err)
		}
		ctx.Progress.Step(commandString(installArgs))
		if output, err := runCommandWithEnv(manifest.Dir, installArgs, sdkPrebuiltEnv(ctx)); err != nil {
			return fmt.Errorf("%s\n%s", err, output)
		}
		report.Notes = append(report.Notes, "cmake install complete")
		return nil
	}

	report.Notes = append(report.Notes, "cmake build complete")
	return nil
}

func (r cmakeRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := r.build(manifest, ctx, report); err != nil {
		return err
	}

	config := cmakeBuildConfig(ctx.Mode)
	listArgs := []string{"ctest", "--test-dir", manifest.CMakeBuildDir(), "-N", "-C", config}
	report.Commands = append(report.Commands, commandString(listArgs))
	ctx.Progress.Step(commandString(listArgs))
	listOutput, err := runCommandWithEnv(manifest.Dir, listArgs, sdkPrebuiltEnv(ctx))
	if err != nil {
		return fmt.Errorf("%s\n%s", err, listOutput)
	}
	if strings.Contains(listOutput, "Total Tests: 0") {
		return fmt.Errorf("no tests configured for cmake runner; register tests with enable_testing() and add_test()")
	}

	testArgs := []string{"ctest", "--test-dir", manifest.CMakeBuildDir(), "--output-on-failure", "-C", config}
	report.Commands = append(report.Commands, commandString(testArgs))
	ctx.Progress.Step(commandString(testArgs))
	if output, err := runCommandWithEnv(manifest.Dir, testArgs, sdkPrebuiltEnv(ctx)); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}

	report.Notes = append(report.Notes, "ctest passed")
	return nil
}

func (cmakeRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

func cmakeProjectUsesInstallRules(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".op", ".build", "build", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if name != "CMakeLists.txt" && filepath.Ext(name) != ".cmake" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(strings.ToLower(string(data)), "install(") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

// --- Recipe runner (composite holons) ---

type recipeRunner struct{}

type recipeBundleSigner struct {
	artifactPath string
	artifactRef  string
	signable     bool
	attempted    bool
}

var bundleSigningHostPlatform = runtimePlatform

var runBundleCodesign = func(dir, artifactRef string, hardened bool) (string, error) {
	return runCommand(dir, bundleCodesignArgs(artifactRef, hardened))
}

func (recipeRunner) check(manifest *LoadedManifest, ctx BuildContext) error {
	if isAggregateBuildTarget(ctx.Target) {
		if len(manifest.Manifest.Build.Targets) == 0 {
			return fmt.Errorf("no recipe targets declared")
		}
		return nil
	}
	if _, ok := manifest.Manifest.Build.Targets[ctx.Target]; !ok {
		return fmt.Errorf("no recipe target %q", ctx.Target)
	}
	// Verify all member paths exist on disk.
	for _, member := range manifest.Manifest.Build.Members {
		memberDir, err := manifest.ResolveManifestPath(member.Path)
		if err != nil {
			return fmt.Errorf("recipe member %q path: %w", member.ID, err)
		}
		if _, err := os.Stat(memberDir); err != nil {
			return fmt.Errorf("recipe member %q path not found: %s", member.ID, memberDir)
		}
		// If the member is a holon, its holon.proto must exist.
		if member.Type == "holon" {
			if _, err := LoadManifest(memberDir); err != nil {
				return fmt.Errorf("recipe member %q (type=holon) missing %s in %s", member.ID, identity.ProtoManifestFileName, memberDir)
			}
		}
	}
	return nil
}

func (r recipeRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if isAggregateBuildTarget(ctx.Target) {
		targets := sortedRecipeTargets(manifest)
		if len(targets) == 0 {
			return fmt.Errorf("no recipe targets declared")
		}

		for _, name := range targets {
			ctx.Progress.Step("building target: " + name)
			childReport, err := ExecuteLifecycle(OperationBuild, manifest.Dir, BuildOptions{
				Target:   name,
				Mode:     ctx.Mode,
				Hardened: ctx.Hardened,
				DryRun:   ctx.DryRun,
				NoSign:   ctx.NoSign,
				Progress: ctx.Progress.Child(),
			})
			report.Children = append(report.Children, childReport)
			if err != nil {
				return fmt.Errorf("recipe target %q: %w", name, err)
			}
		}

		report.Notes = append(report.Notes, fmt.Sprintf("recipe aggregate build covered targets: %s", strings.Join(targets, ", ")))
		return nil
	}

	target, ok := manifest.Manifest.Build.Targets[ctx.Target]
	if !ok {
		available := make([]string, 0, len(manifest.Manifest.Build.Targets))
		for name := range manifest.Manifest.Build.Targets {
			available = append(available, name)
		}
		return fmt.Errorf("no recipe target %q (available: %s)", ctx.Target, strings.Join(available, ", "))
	}

	memberMap := make(map[string]RecipeMember, len(manifest.Manifest.Build.Members))
	for _, m := range manifest.Manifest.Build.Members {
		memberMap[m.ID] = m
	}
	excludedMembers := resolveHardenedExcludedMembers(manifest, ctx)

	signer := newRecipeBundleSigner(manifest, ctx)
	for i, step := range target.Steps {
		if step.AssertFile != nil {
			if err := r.signBundleArtifact(manifest, ctx, report, signer); err != nil {
				return fmt.Errorf("target %q signing: %w", ctx.Target, err)
			}
		}
		stepLabel := fmt.Sprintf("step %d", i+1)
		if reason := recipeExcludedStepReason(step, excludedMembers); reason != "" {
			ctx.Progress.Step(reason)
			report.Notes = append(report.Notes, reason)
			continue
		}
		if err := r.executeStep(manifest, ctx, step, memberMap, report, stepLabel); err != nil {
			return fmt.Errorf("target %q %s: %w", ctx.Target, stepLabel, err)
		}
	}
	if err := r.signBundleArtifact(manifest, ctx, report, signer); err != nil {
		return fmt.Errorf("target %q signing: %w", ctx.Target, err)
	}

	if !ctx.DryRun {
		report.Notes = append(report.Notes, fmt.Sprintf("recipe target %q completed (%d steps)", ctx.Target, len(target.Steps)))
	}
	return nil
}

func (r recipeRunner) executeStep(manifest *LoadedManifest, ctx BuildContext, step RecipeStep, members map[string]RecipeMember, report *Report, label string) error {
	switch {
	case step.BuildMember != "":
		return r.stepBuildMember(manifest, ctx, step.BuildMember, members, report)
	case step.Exec != nil:
		return r.stepExec(manifest, ctx, step.Exec, report)
	case step.Copy != nil:
		return r.stepCopy(manifest, ctx, step.Copy, report)
	case step.CopyArtifact != nil:
		return r.stepCopyArtifact(manifest, ctx, step.CopyArtifact, members, report)
	case step.CopyAllHolons != nil:
		return r.stepCopyAllHolons(manifest, ctx, step.CopyAllHolons, report)
	case step.AssertFile != nil:
		return r.stepAssertFile(manifest, ctx, step.AssertFile, report)
	default:
		return fmt.Errorf("%s: empty step (no action defined)", label)
	}
}

// stepBuildMember recursively builds a child holon.
func (recipeRunner) stepBuildMember(manifest *LoadedManifest, ctx BuildContext, memberID string, members map[string]RecipeMember, report *Report) error {
	member, ok := members[memberID]
	if !ok {
		return fmt.Errorf("unknown member %q", memberID)
	}
	if member.Type != "holon" {
		return fmt.Errorf("build_member can only target holon members, %q is %q", memberID, member.Type)
	}

	memberDir, err := manifest.ResolveManifestPath(member.Path)
	if err != nil {
		return fmt.Errorf("build_member %q path: %w", memberID, err)
	}
	if resolved, err := filepath.EvalSymlinks(memberDir); err == nil {
		memberDir = resolved
	}
	report.Commands = append(report.Commands, "build_member "+memberID)

	if ctx.composite != nil && ctx.composite.skipDependencyPrebuild {
		node, nodeErr := compositeNodeForDir(memberDir, ctx, ctx.composite.session)
		if nodeErr != nil {
			return fmt.Errorf("build_member %q manifest: %w", memberID, nodeErr)
		}
		if _, ok := ctx.composite.session.completed[node.key]; !ok {
			return fmt.Errorf("build_member %q has no prebuilt artifact", memberID)
		}
		if !ctx.DryRun {
			report.Notes = append(report.Notes, fmt.Sprintf("used prebuilt member %q", memberID))
		}
		return nil
	}

	ctx.Progress.Step("building member: " + memberID)
	childReport, err := ExecuteLifecycle(OperationBuild, memberDir, BuildOptions{
		Target:   ctx.Target,
		Mode:     ctx.Mode,
		Hardened: ctx.Hardened,
		DryRun:   ctx.DryRun,
		NoSign:   ctx.NoSign,
		Progress: ctx.Progress.Child(),
	})
	report.Children = append(report.Children, childReport)
	if err != nil {
		return fmt.Errorf("build_member %q: %w", memberID, err)
	}

	if !ctx.DryRun {
		report.Notes = append(report.Notes, fmt.Sprintf("built member %q", memberID))
	}
	return nil
}

// stepExec runs an argv command in an explicit working directory.
func (recipeRunner) stepExec(manifest *LoadedManifest, ctx BuildContext, e *RecipeStepExec, report *Report) error {
	if len(e.Argv) == 0 {
		return fmt.Errorf("exec step has empty argv")
	}

	cwd, err := manifest.ResolveManifestPath(e.Cwd)
	if err != nil {
		return fmt.Errorf("exec cwd %q: %w", e.Cwd, err)
	}
	report.Commands = append(report.Commands, fmt.Sprintf("(cwd=%s) %s", manifestRelativePath(manifest, cwd), commandString(e.Argv)))
	ctx.Progress.Step(commandString(e.Argv))
	if ctx.DryRun {
		return nil
	}
	if output, err := runCommandWithEnv(cwd, e.Argv, buildExecutionEnv(manifest, ctx)); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}

	return nil
}

// stepCopy copies a file from one manifest-relative path to another.
func (recipeRunner) stepCopy(manifest *LoadedManifest, ctx BuildContext, c *RecipeStepCopy, report *Report) error {
	src, err := manifest.ResolveManifestPath(c.From)
	if err != nil {
		return fmt.Errorf("copy from %q: %w", c.From, err)
	}
	dst, err := manifest.ResolveManifestPath(c.To)
	if err != nil {
		return fmt.Errorf("copy to %q: %w", c.To, err)
	}
	report.Commands = append(report.Commands, fmt.Sprintf("copy %s -> %s", c.From, c.To))
	ctx.Progress.Step(fmt.Sprintf("copy %s -> %s", c.From, c.To))
	if ctx.DryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("copy: create dir for %s: %w", c.To, err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("copy: read %s: %w", c.From, err)
	}
	// Preserve the source file's permissions.
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy: stat %s: %w", c.From, err)
	}
	if err := os.WriteFile(dst, data, info.Mode()); err != nil {
		return fmt.Errorf("copy: write %s: %w", c.To, err)
	}

	report.Notes = append(report.Notes, fmt.Sprintf("copied %s → %s", c.From, c.To))
	return nil
}

func (recipeRunner) stepCopyArtifact(manifest *LoadedManifest, ctx BuildContext, ca *RecipeStepCopyArtifact, members map[string]RecipeMember, report *Report) error {
	member, ok := members[ca.From]
	if !ok {
		return fmt.Errorf("copy_artifact: unknown member %q", ca.From)
	}

	memberDir, err := manifest.ResolveManifestPath(member.Path)
	if err != nil {
		return fmt.Errorf("copy_artifact member %q path: %w", ca.From, err)
	}
	memberManifest, err := LoadManifest(memberDir)
	if err != nil {
		return fmt.Errorf("copy_artifact member %q manifest: %w", ca.From, err)
	}

	srcDir := availableHolonPackageDir(memberManifest)
	dstDir, err := manifest.ResolveManifestPath(ca.To)
	if err != nil {
		return fmt.Errorf("copy_artifact to %q: %w", ca.To, err)
	}

	report.Commands = append(report.Commands, fmt.Sprintf("copy_artifact %s -> %s", ca.From, ca.To))
	ctx.Progress.Step(fmt.Sprintf("copy_artifact %s -> %s", ca.From, ca.To))
	if ctx.DryRun {
		return nil
	}

	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("copy_artifact source missing for %q: %s", ca.From, workspaceRelativePath(srcDir))
	}
	if err := copyArtifact(srcDir, dstDir); err != nil {
		return err
	}

	report.Notes = append(report.Notes, fmt.Sprintf("copied artifact %s → %s", ca.From, ca.To))
	return nil
}

func (recipeRunner) stepCopyAllHolons(manifest *LoadedManifest, ctx BuildContext, ca *RecipeStepCopyAllHolons, report *Report) error {
	dstRoot, err := manifest.ResolveManifestPath(ca.To)
	if err != nil {
		return fmt.Errorf("copy_all_holons to %q: %w", ca.To, err)
	}

	report.Commands = append(report.Commands, fmt.Sprintf("copy_all_holons -> %s", ca.To))
	ctx.Progress.Step(fmt.Sprintf("copy_all_holons -> %s", ca.To))
	if ctx.DryRun {
		return nil
	}

	if err := os.RemoveAll(dstRoot); err != nil {
		return fmt.Errorf("copy_all_holons: clear destination %s: %w", workspaceRelativePath(dstRoot), err)
	}
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		return fmt.Errorf("copy_all_holons: create destination %s: %w", workspaceRelativePath(dstRoot), err)
	}

	seen := make(map[string]string)
	visiting := map[string]bool{compositeDependencyKey(manifest): true}

	var collect func(*LoadedManifest) error
	collect = func(parent *LoadedManifest) error {
		excludedCtx := ctx
		excludedCtx.Progress = nil
		excludedMembers := resolveHardenedExcludedMembers(parent, excludedCtx)
		for _, member := range parent.Manifest.Build.Members {
			if member.Type != "holon" {
				continue
			}
			if runnerName, excluded := excludedMembers[member.ID]; excluded {
				note := fmt.Sprintf("hardened: skipped copy_all_holons member %q (runner %q not standalone)", member.ID, runnerName)
				ctx.Progress.Step(note)
				report.Notes = append(report.Notes, note)
				continue
			}
			memberDir, err := parent.ResolveManifestPath(member.Path)
			if err != nil {
				return fmt.Errorf("copy_all_holons member %q path: %w", member.ID, err)
			}
			memberManifest, err := LoadManifest(memberDir)
			if err != nil {
				return fmt.Errorf("copy_all_holons member %q manifest: %w", member.ID, err)
			}
			if resolved, err := filepath.EvalSymlinks(memberManifest.Dir); err == nil {
				memberManifest.Dir = resolved
			}

			slug := manifestSlug(memberManifest)
			if memberManifest.Manifest.Kind == KindComposite {
				key := compositeDependencyKey(memberManifest)
				if visiting[key] {
					return fmt.Errorf("copy_all_holons: composite dependency cycle at %q", slug)
				}
			}
			srcDir := availableHolonPackageDir(memberManifest)
			if info, err := os.Stat(srcDir); err != nil || !info.IsDir() {
				return fmt.Errorf("copy_all_holons source missing for %q: %s", slug, workspaceRelativePath(srcDir))
			}
			if existingSrc, dup := seen[slug]; dup {
				return fmt.Errorf("copy_all_holons: slug collision %q from %s and %s", slug, workspaceRelativePath(existingSrc), workspaceRelativePath(srcDir))
			}
			seen[slug] = srcDir

			dstDir := filepath.Join(dstRoot, slug+".holon")
			if err := os.RemoveAll(dstDir); err != nil {
				return fmt.Errorf("copy_all_holons: clear destination %s: %w", workspaceRelativePath(dstDir), err)
			}
			if err := copyDir(srcDir, dstDir); err != nil {
				return fmt.Errorf("copy_all_holons: copy %s: %w", slug, err)
			}

			if memberManifest.Manifest.Kind == KindComposite {
				key := compositeDependencyKey(memberManifest)
				visiting[key] = true
				if err := collect(memberManifest); err != nil {
					return err
				}
				delete(visiting, key)
			}
		}
		return nil
	}
	if err := collect(manifest); err != nil {
		return err
	}

	report.Notes = append(report.Notes, fmt.Sprintf("copied %d holon artifacts → %s", len(seen), ca.To))
	return nil
}

// stepAssertFile verifies a manifest-relative file exists.
func (recipeRunner) stepAssertFile(manifest *LoadedManifest, ctx BuildContext, a *RecipeStepFile, report *Report) error {
	path, err := manifest.ResolveManifestPath(a.Path)
	if err != nil {
		return fmt.Errorf("assert_file path %q: %w", a.Path, err)
	}
	report.Commands = append(report.Commands, "assert_file "+a.Path)
	ctx.Progress.Step("assert_file " + a.Path)
	if ctx.DryRun {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("assert_file: expected %s but not found", a.Path)
	}
	report.Notes = append(report.Notes, fmt.Sprintf("verified %s", a.Path))
	return nil
}

func (recipeRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	// Run op test on each holon member.
	for _, member := range manifest.Manifest.Build.Members {
		if member.Type != "holon" {
			continue
		}
		memberDir, err := manifest.ResolveManifestPath(member.Path)
		if err != nil {
			return fmt.Errorf("test member %q path: %w", member.ID, err)
		}
		childReport, err := ExecuteLifecycle(OperationTest, memberDir, BuildOptions{
			Target: ctx.Target,
			Mode:   ctx.Mode,
		})
		report.Children = append(report.Children, childReport)
		if err != nil {
			return fmt.Errorf("test member %q: %w", member.ID, err)
		}
	}
	report.Notes = append(report.Notes, "all holon members tested")
	return nil
}

func (recipeRunner) clean(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	writer := progress.WriterFromReporter(ctx.Progress)
	for _, member := range manifest.Manifest.Build.Members {
		if member.Type != "holon" {
			continue
		}

		memberDir, err := manifest.ResolveManifestPath(member.Path)
		if err != nil {
			return fmt.Errorf("clean member %q path: %w", member.ID, err)
		}
		if resolved, err := filepath.EvalSymlinks(memberDir); err == nil {
			memberDir = resolved
		}

		memberManifest, err := LoadManifest(memberDir)
		if err != nil {
			return fmt.Errorf("clean member %q manifest: %w", member.ID, err)
		}

		cleanStart := time.Now()
		childReport, err := ExecuteLifecycle(OperationClean, memberDir, BuildOptions{
			Progress: ctx.Progress.Child(),
		})
		if err != nil {
			report.Children = append(report.Children, childReport)
			return fmt.Errorf("clean member %q: %w", member.ID, err)
		}
		if err := cleanSharedHolonPackageCache(memberManifest, &childReport); err != nil {
			report.Children = append(report.Children, childReport)
			return fmt.Errorf("clean member %q shared cache: %w", member.ID, err)
		}
		report.Children = append(report.Children, childReport)
		if writer != nil {
			writer.FreezeAt(cleanSuccessLine(childReport.Holon, time.Since(cleanStart)), cleanStart)
		}
	}

	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

func runCommand(dir string, args []string) (string, error) {
	return runCommandWithEnv(dir, args, nil)
}

func runCommandWithEnv(dir string, args []string, extraEnv []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if len(extraEnv) != 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed: %w", commandString(args), err)
	}
	return string(output), nil
}

func recipeExecEnv(manifest *LoadedManifest, ctx BuildContext) []string {
	env := []string{
		"OP_BUILD_TARGET=" + ctx.Target,
		"OP_BUILD_MODE=" + ctx.Mode,
	}
	if manifest != nil {
		if pkgDir := manifest.HolonPackageDir(); pkgDir != "" {
			env = append(env, "OP_PACKAGE_DIR="+pkgDir)
		}
		if binPath := manifest.BinaryPath(); binPath != "" {
			env = append(env, "OP_BINARY_DIR="+filepath.Dir(binPath))
		}
	}
	if ctx.Hardened {
		env = append(env, "OP_BUILD_HARDENED=true")
	} else {
		env = append(env, "OP_BUILD_HARDENED=false")
	}
	return env
}

func compositeMemberPackageEnv(manifest *LoadedManifest) []string {
	if manifest == nil || manifest.Manifest.Kind != KindComposite || len(manifest.Manifest.Build.Members) == 0 {
		return nil
	}
	env := make([]string, 0, len(manifest.Manifest.Build.Members))
	for _, member := range manifest.Manifest.Build.Members {
		if member.Type != "holon" {
			continue
		}
		memberDir, err := manifest.ResolveManifestPath(member.Path)
		if err != nil {
			continue
		}
		memberManifest, err := LoadManifest(memberDir)
		if err != nil {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(memberManifest.Dir); err == nil {
			memberManifest.Dir = resolved
		}
		slug := strings.TrimSpace(manifestSlug(memberManifest))
		if slug == "" {
			continue
		}
		envKey := "OP_HOLON_" + strings.ReplaceAll(strings.ToUpper(slug), "-", "_") + "_PATH"
		env = append(env, envKey+"="+availableHolonPackageDir(memberManifest))
	}
	return env
}

func buildExecutionEnv(manifest *LoadedManifest, ctx BuildContext) []string {
	env := recipeExecEnv(manifest, ctx)
	env = append(env, compositeMemberPackageEnv(manifest)...)
	env = append(env, sdkPrebuiltEnv(ctx)...)
	return env
}

func executePipelineHooks(manifest *LoadedManifest, ctx BuildContext, report *Report, hooks []*RecipeStepExec) error {
	for _, hook := range hooks {
		cwd, err := manifest.ResolveManifestPath(hook.Cwd)
		if err != nil {
			cwd = manifest.Dir
		}

		ctx.Progress.Step(commandString(hook.Argv))
		report.Commands = append(report.Commands, fmt.Sprintf("(cwd=%s) %s", hook.Cwd, commandString(hook.Argv)))

		if !ctx.DryRun {
			if out, err := runCommandWithEnv(cwd, hook.Argv, buildExecutionEnv(manifest, ctx)); err != nil {
				return fmt.Errorf("hook %s failed: %s\n%s", commandString(hook.Argv), err, out)
			}
		}
	}
	return nil
}

func commandString(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t") {
			quoted = append(quoted, fmt.Sprintf("%q", arg))
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}

func newRecipeBundleSigner(manifest *LoadedManifest, ctx BuildContext) *recipeBundleSigner {
	artifactPath := manifest.ArtifactPath(ctx)
	if artifactPath == "" {
		return &recipeBundleSigner{}
	}
	return &recipeBundleSigner{
		artifactPath: artifactPath,
		artifactRef:  manifestRelativePath(manifest, artifactPath),
		signable:     isSignableBundleArtifact(artifactPath),
	}
}

func (recipeRunner) signBundleArtifact(manifest *LoadedManifest, ctx BuildContext, report *Report, signer *recipeBundleSigner) error {
	if signer == nil || signer.attempted || !signer.signable {
		return nil
	}
	signer.attempted = true

	if ctx.NoSign {
		report.Notes = append(report.Notes, "skip signing (--no-sign): "+signer.artifactRef)
		return nil
	}
	if bundleSigningHostPlatform() != "darwin" {
		report.Notes = append(report.Notes, "skip signing: not on macOS: "+signer.artifactRef)
		return nil
	}

	args := bundleCodesignArgs(signer.artifactRef, ctx.Hardened)
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if ctx.DryRun {
		return nil
	}
	if output, err := runBundleCodesign(manifest.Dir, signer.artifactRef, ctx.Hardened); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}

	report.Notes = append(report.Notes, "signed (ad-hoc): "+signer.artifactRef)
	return nil
}

func isSignableBundleArtifact(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lower, ".app") || strings.HasSuffix(lower, ".framework")
}

func bundleCodesignArgs(artifactRef string, hardened bool) []string {
	args := []string{"codesign", "--force", "--deep"}
	if hardened {
		args = append(args, "--preserve-metadata=entitlements")
	}
	args = append(args, "--sign", "-", artifactRef)
	return args
}

func normalizedTarget(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "."
	}
	return ref
}

func normalizeBuildTarget(target string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(target))
	switch normalized {
	case "darwin", "macos":
		return "macos", nil
	case "linux", "windows", "ios", "ios-simulator", "tvos", "tvos-simulator", "watchos", "watchos-simulator", "visionos", "visionos-simulator", "android", "all":
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported target %q (supported: macos, linux, windows, ios, ios-simulator, tvos, tvos-simulator, watchos, watchos-simulator, visionos, visionos-simulator, android, all)", target)
	}
}

func normalizePlatformName(platform string) string {
	normalized, err := normalizeBuildTarget(platform)
	if err == nil {
		return normalized
	}
	return strings.ToLower(strings.TrimSpace(platform))
}

func normalizeBuildMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func isValidBuildMode(mode string) bool {
	switch normalizeBuildMode(mode) {
	case buildModeDebug, buildModeRelease, buildModeProfile:
		return true
	default:
		return false
	}
}

func isAggregateBuildTarget(target string) bool {
	return strings.EqualFold(strings.TrimSpace(target), "all")
}

func sortedRecipeTargets(manifest *LoadedManifest) []string {
	targets := make([]string, 0, len(manifest.Manifest.Build.Targets))
	for name := range manifest.Manifest.Build.Targets {
		targets = append(targets, name)
	}

	order := map[string]int{
		"macos":              10,
		"ios":                20,
		"ios-simulator":      21,
		"tvos":               30,
		"tvos-simulator":     31,
		"watchos":            40,
		"watchos-simulator":  41,
		"visionos":           50,
		"visionos-simulator": 51,
		"android":            60,
		"linux":              70,
		"windows":            80,
	}

	sort.Slice(targets, func(i, j int) bool {
		left := order[targets[i]]
		right := order[targets[j]]
		if left != right {
			return left < right
		}
		return targets[i] < targets[j]
	})
	return targets
}

func canonicalRuntimeTarget() string {
	switch runtimePlatform() {
	case "darwin":
		return "macos"
	default:
		return runtimePlatform()
	}
}

func ensureHostBuildTarget(runnerName string, ctx BuildContext) error {
	if ctx.Target == canonicalRuntimeTarget() {
		return nil
	}
	return fmt.Errorf("%s cross-target build not implemented (requested %q on host %q)", runnerName, ctx.Target, canonicalRuntimeTarget())
}

func cmakeBuildConfig(mode string) string {
	switch normalizeBuildMode(mode) {
	case buildModeRelease:
		return "Release"
	case buildModeProfile:
		return "RelWithDebInfo"
	default:
		return "Debug"
	}
}

func manifestRelativePath(manifest *LoadedManifest, absPath string) string {
	rel, err := filepath.Rel(manifest.Dir, absPath)
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	return filepath.ToSlash(rel)
}

func installHint(command string) string {
	switch runtimePlatform() {
	case "darwin":
		switch command {
		case "ctest":
			return "install CMake, which provides ctest (`brew install cmake`)"
		default:
			return fmt.Sprintf("install it with Homebrew if available (`brew install %s`)", command)
		}
	case "linux":
		switch command {
		case "ctest":
			return "install CMake, which provides ctest, with your distribution package manager"
		default:
			return fmt.Sprintf("install it with your distribution package manager (for example `%s`)", linuxInstallExample(command))
		}
	case "windows":
		switch command {
		case "ctest":
			return "install CMake and ensure `ctest.exe` is on PATH"
		default:
			return fmt.Sprintf("install %s and ensure it is on PATH", command)
		}
	default:
		return fmt.Sprintf("install %s and ensure it is on PATH", command)
	}
}

func linuxInstallExample(command string) string {
	switch command {
	case "go":
		return "sudo apt install golang-go"
	case "cmake", "ctest":
		return "sudo apt install cmake"
	default:
		return "sudo apt install " + command
	}
}

func runtimePlatform() string {
	return runtime.GOOS
}
