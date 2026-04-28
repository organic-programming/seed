package holons

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/organic-programming/grace-op/internal/progress"
	"github.com/organic-programming/grace-op/internal/sdkprebuilts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const codegenManifestName = "codegen-manifest.json"

var knownCodegenSDKSlugs = map[string]struct{}{
	"c":      {},
	"cpp":    {},
	"csharp": {},
	"dart":   {},
	"go":     {},
	"java":   {},
	"js":     {},
	"js-web": {},
	"kotlin": {},
	"python": {},
	"ruby":   {},
	"rust":   {},
	"swift":  {},
	"zig":    {},
}

type resolvedCodegenPlugin struct {
	Name      string
	SDK       string
	Version   string
	Target    string
	Root      string
	Binary    string
	OutSubdir string
}

type emittedCodegenFile struct {
	Plugin    string
	OutSubdir string
	Name      string
	Content   []byte
}

type codegenDistributionManifest struct {
	Lang    string `json:"lang"`
	Version string `json:"version"`
	Target  string `json:"target"`
	Codegen struct {
		Plugins []codegenDistributionPlugin `json:"plugins"`
	} `json:"codegen"`
}

type codegenDistributionPlugin struct {
	Name      string `json:"name"`
	Binary    string `json:"binary"`
	OutSubdir string `json:"out_subdir"`
}

type codegenCacheManifest struct {
	Schema  string               `json:"schema"`
	Files   []codegenCacheFile   `json:"files"`
	Plugins []codegenCachePlugin `json:"plugins"`
}

type codegenCachePlugin struct {
	Name      string `json:"name"`
	SDK       string `json:"sdk"`
	Version   string `json:"version"`
	Target    string `json:"target"`
	Binary    string `json:"binary"`
	OutSubdir string `json:"out_subdir"`
}

type codegenCacheFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Plugin string `json:"plugin"`
}

func runCodegen(manifest *LoadedManifest, ctx BuildContext, stage *protoStageResult, reporter progress.Reporter, report *Report) error {
	languages := normalizedCodegenLanguages(manifest)
	if len(languages) == 0 {
		return nil
	}

	plugins, err := resolveCodegenPlugins(languages)
	if err != nil {
		return err
	}

	if ctx.DryRun {
		for _, plugin := range plugins {
			note := fmt.Sprintf("codegen: %s -> gen/%s using %s", plugin.Name, filepath.ToSlash(plugin.OutSubdir), plugin.Binary)
			report.Notes = append(report.Notes, note)
		}
		reporter.Step("codegen: planned")
		return nil
	}

	if !stage.hasProtos() {
		return fmt.Errorf("codegen: build.codegen.languages declared but no proto files were staged")
	}

	reqBytes, err := buildCodegenRequest(stage)
	if err != nil {
		return err
	}

	reporter.Step("codegen: running plugins...")
	emitted, err := invokeCodegenPlugins(context.Background(), plugins, reqBytes)
	if err != nil {
		return err
	}

	written, removed, err := writeCodegenOutputs(manifest, plugins, emitted)
	if err != nil {
		return err
	}
	reporter.Step(fmt.Sprintf("codegen: wrote %d file(s)", written))
	if removed > 0 {
		reporter.Step(fmt.Sprintf("codegen: removed %d stale file(s)", removed))
	}
	report.Notes = append(report.Notes, fmt.Sprintf("codegen: wrote %d file(s)", written))
	return nil
}

func validateCodegenPlugins(manifest *LoadedManifest) error {
	languages := normalizedCodegenLanguages(manifest)
	if len(languages) == 0 {
		return nil
	}
	_, err := resolveCodegenPlugins(languages)
	return err
}

func normalizedCodegenLanguages(manifest *LoadedManifest) []string {
	if manifest == nil {
		return nil
	}
	values := manifest.Manifest.Build.Codegen.Languages
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
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

func buildCodegenRequest(stage *protoStageResult) ([]byte, error) {
	if stage == nil {
		return nil, fmt.Errorf("codegen: proto stage result required")
	}

	files := make(map[string]*descriptorpb.FileDescriptorProto)
	var collect func(protoreflect.FileDescriptor)
	collect = func(fd protoreflect.FileDescriptor) {
		if fd == nil {
			return
		}
		if _, ok := files[fd.Path()]; ok {
			return
		}
		imports := fd.Imports()
		for i := 0; i < imports.Len(); i++ {
			collect(imports.Get(i).FileDescriptor)
		}
		files[fd.Path()] = protodesc.ToFileDescriptorProto(fd)
	}
	for _, fd := range stage.Files {
		collect(fd)
	}

	protoPaths := make([]string, 0, len(files))
	for path := range files {
		protoPaths = append(protoPaths, path)
	}
	sort.Strings(protoPaths)

	toGenerate := append([]string(nil), stage.HolonProtos...)
	for i := range toGenerate {
		toGenerate[i] = filepath.ToSlash(toGenerate[i])
	}
	sort.Strings(toGenerate)

	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: toGenerate,
		ProtoFile:      make([]*descriptorpb.FileDescriptorProto, 0, len(protoPaths)),
	}
	for _, path := range protoPaths {
		req.ProtoFile = append(req.ProtoFile, files[path])
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("codegen: marshal request: %w", err)
	}
	return data, nil
}

func resolveCodegenPlugins(languages []string) ([]resolvedCodegenPlugin, error) {
	target, err := sdkprebuilts.HostTriplet()
	if err != nil {
		return nil, err
	}

	plugins := make([]resolvedCodegenPlugin, 0, len(languages))
	for _, language := range languages {
		plugin, err := resolveCodegenPlugin(language, target)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

func resolveCodegenPlugin(language, target string) (resolvedCodegenPlugin, error) {
	sdk := codegenSDKSlug(language)
	root, dist, err := locateCodegenDistribution(sdk, target)
	if err != nil {
		if errors.Is(err, errCodegenDistributionMissing) {
			return resolvedCodegenPlugin{}, fmt.Errorf("missing distribution for codegen language %q: SDK %q for %s is not installed; action: op sdk install %s", language, sdk, target, sdk)
		}
		return resolvedCodegenPlugin{}, err
	}

	declared := make([]string, 0, len(dist.Codegen.Plugins))
	for _, plugin := range dist.Codegen.Plugins {
		name := strings.TrimSpace(plugin.Name)
		if name != "" {
			declared = append(declared, name)
		}
		if name != language {
			continue
		}
		binary, err := safeCodegenJoin(root, plugin.Binary)
		if err != nil {
			return resolvedCodegenPlugin{}, fmt.Errorf("codegen plugin %q binary: %w", language, err)
		}
		if err := validateCodegenBinary(binary); err != nil {
			return resolvedCodegenPlugin{}, fmt.Errorf("codegen plugin %q binary: %w", language, err)
		}
		outSubdir := strings.TrimSpace(plugin.OutSubdir)
		if outSubdir == "" {
			return resolvedCodegenPlugin{}, fmt.Errorf("codegen plugin %q has empty out_subdir", language)
		}
		if _, err := safeCodegenJoin(string(filepath.Separator), outSubdir); err != nil {
			return resolvedCodegenPlugin{}, fmt.Errorf("codegen plugin %q out_subdir: %w", language, err)
		}
		return resolvedCodegenPlugin{
			Name:      language,
			SDK:       sdk,
			Version:   strings.TrimSpace(dist.Version),
			Target:    strings.TrimSpace(dist.Target),
			Root:      root,
			Binary:    binary,
			OutSubdir: outSubdir,
		}, nil
	}
	sort.Strings(declared)
	return resolvedCodegenPlugin{}, fmt.Errorf("unsupported codegen language %q in SDK %q; distribution declares: %s", language, sdk, formatDeclaredCodegenPlugins(declared))
}

var errCodegenDistributionMissing = errors.New("codegen distribution missing")

func locateCodegenDistribution(sdk, target string) (string, codegenDistributionManifest, error) {
	langDir := filepath.Join(sdkprebuilts.SDKRoot(), sdk)
	entries, err := os.ReadDir(langDir)
	if errors.Is(err, os.ErrNotExist) {
		return "", codegenDistributionManifest{}, errCodegenDistributionMissing
	}
	if err != nil {
		return "", codegenDistributionManifest{}, err
	}

	type candidate struct {
		version string
		root    string
	}
	candidates := make([]candidate, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(langDir, entry.Name(), target)
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			candidates = append(candidates, candidate{version: entry.Name(), root: root})
		}
	}
	if len(candidates) == 0 {
		return "", codegenDistributionManifest{}, errCodegenDistributionMissing
	}
	sort.Slice(candidates, func(i, j int) bool {
		return compareCodegenVersion(candidates[i].version, candidates[j].version) > 0
	})

	root := candidates[0].root
	dist, err := readCodegenDistributionManifest(root)
	if err != nil {
		return "", codegenDistributionManifest{}, err
	}
	if strings.TrimSpace(dist.Version) == "" {
		dist.Version = candidates[0].version
	}
	if strings.TrimSpace(dist.Target) == "" {
		dist.Target = target
	}
	if strings.TrimSpace(dist.Lang) == "" {
		dist.Lang = sdk
	}
	return root, dist, nil
}

func readCodegenDistributionManifest(root string) (codegenDistributionManifest, error) {
	path := filepath.Join(root, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return codegenDistributionManifest{}, fmt.Errorf("read codegen distribution manifest %s: %w", path, err)
	}
	var manifest codegenDistributionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return codegenDistributionManifest{}, fmt.Errorf("parse codegen distribution manifest %s: %w", path, err)
	}
	return manifest, nil
}

func codegenSDKSlug(language string) string {
	if _, ok := knownCodegenSDKSlugs[language]; ok {
		return language
	}
	if before, _, ok := strings.Cut(language, "-"); ok && strings.TrimSpace(before) != "" {
		return strings.TrimSpace(before)
	}
	return language
}

func validateCodegenBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return fmt.Errorf("%s is not executable", path)
	}
	return nil
}

func invokeCodegenPlugins(ctx context.Context, plugins []resolvedCodegenPlugin, reqBytes []byte) ([]emittedCodegenFile, error) {
	type result struct {
		files []emittedCodegenFile
		err   error
	}

	results := make(chan result, len(plugins))
	var wg sync.WaitGroup
	for _, plugin := range plugins {
		plugin := plugin
		wg.Add(1)
		go func() {
			defer wg.Done()
			files, err := invokeCodegenPlugin(ctx, plugin, reqBytes)
			results <- result{files: files, err: err}
		}()
	}
	wg.Wait()
	close(results)

	var emitted []emittedCodegenFile
	for result := range results {
		if result.err != nil {
			return nil, result.err
		}
		emitted = append(emitted, result.files...)
	}
	return emitted, nil
}

func invokeCodegenPlugin(ctx context.Context, plugin resolvedCodegenPlugin, reqBytes []byte) ([]emittedCodegenFile, error) {
	tmp, err := os.MkdirTemp("", "op-codegen-*")
	if err != nil {
		return nil, fmt.Errorf("codegen plugin failed: %s: create temp dir: %w", plugin.Name, err)
	}
	defer os.RemoveAll(tmp)

	cmd := exec.CommandContext(ctx, plugin.Binary)
	cmd.Dir = tmp
	cmd.Stdin = bytes.NewReader(reqBytes)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codegen plugin failed: %s: exec: %w", plugin.Name, err)
	}

	resp := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(stdout.Bytes(), resp); err != nil {
		return nil, fmt.Errorf("codegen plugin failed: %s: decode response: %w", plugin.Name, err)
	}
	if errMsg := strings.TrimSpace(resp.GetError()); errMsg != "" {
		return nil, fmt.Errorf("codegen plugin failed: %s: %s", plugin.Name, errMsg)
	}

	files := make([]emittedCodegenFile, 0, len(resp.File))
	for _, file := range resp.File {
		if strings.TrimSpace(file.GetInsertionPoint()) != "" {
			return nil, fmt.Errorf("codegen plugin failed: %s: insertion points are not supported for %s", plugin.Name, file.GetName())
		}
		files = append(files, emittedCodegenFile{
			Plugin:    plugin.Name,
			OutSubdir: plugin.OutSubdir,
			Name:      file.GetName(),
			Content:   []byte(file.GetContent()),
		})
	}
	return files, nil
}

func writeCodegenOutputs(manifest *LoadedManifest, plugins []resolvedCodegenPlugin, emitted []emittedCodegenFile) (int, int, error) {
	if manifest == nil {
		return 0, 0, fmt.Errorf("codegen: manifest required")
	}

	type pendingFile struct {
		rel     string
		abs     string
		plugin  string
		content []byte
	}
	pending := make(map[string]pendingFile, len(emitted))
	for _, file := range emitted {
		abs, rel, err := codegenOutputPath(manifest, file.OutSubdir, file.Name)
		if err != nil {
			return 0, 0, err
		}
		if previous, ok := pending[rel]; ok {
			return 0, 0, fmt.Errorf("codegen plugin failed: duplicate output path %s from %s and %s", rel, previous.plugin, file.Plugin)
		}
		pending[rel] = pendingFile{
			rel:     rel,
			abs:     abs,
			plugin:  file.Plugin,
			content: append([]byte(nil), file.Content...),
		}
	}

	cache, err := readCodegenCache(manifest)
	if err != nil {
		return 0, 0, err
	}
	previous := make(map[string]codegenCacheFile, len(cache.Files))
	for _, file := range cache.Files {
		previous[file.Path] = file
	}

	removed := 0
	for rel, file := range previous {
		if _, ok := pending[rel]; ok {
			continue
		}
		abs := filepath.Join(manifest.Dir, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, 0, fmt.Errorf("codegen: read stale output %s: %w", rel, err)
		}
		if file.SHA256 != "" && sha256Hex(data) != file.SHA256 {
			return 0, 0, fmt.Errorf("codegen deleted unexpected file: %s changed since the previous codegen run", rel)
		}
		if err := os.Remove(abs); err != nil {
			return 0, 0, fmt.Errorf("codegen: remove stale output %s: %w", rel, err)
		}
		removed++
	}

	paths := make([]string, 0, len(pending))
	for rel := range pending {
		paths = append(paths, rel)
	}
	sort.Strings(paths)

	files := make([]codegenCacheFile, 0, len(paths))
	for _, rel := range paths {
		file := pending[rel]
		if err := os.MkdirAll(filepath.Dir(file.abs), 0o755); err != nil {
			return 0, 0, fmt.Errorf("codegen: create output dir for %s: %w", rel, err)
		}
		if err := os.WriteFile(file.abs, file.content, 0o644); err != nil {
			return 0, 0, fmt.Errorf("codegen: write %s: %w", rel, err)
		}
		files = append(files, codegenCacheFile{
			Path:   rel,
			SHA256: sha256Hex(file.content),
			Plugin: file.plugin,
		})
	}

	next := codegenCacheManifest{
		Schema:  "op-codegen-manifest/v1",
		Files:   files,
		Plugins: codegenCachePlugins(plugins),
	}
	if err := writeCodegenCache(manifest, next); err != nil {
		return 0, 0, err
	}
	return len(files), removed, nil
}

func codegenOutputPath(manifest *LoadedManifest, outSubdir, name string) (string, string, error) {
	outRoot, err := safeCodegenJoin(filepath.Join(manifest.Dir, "gen"), outSubdir)
	if err != nil {
		return "", "", fmt.Errorf("codegen path escaped out_dir: %s: %w", outSubdir, err)
	}
	abs, err := safeCodegenJoin(outRoot, name)
	if err != nil {
		return "", "", fmt.Errorf("codegen path escaped out_dir: %s: %w", name, err)
	}
	rel, err := filepath.Rel(manifest.Dir, abs)
	if err != nil {
		return "", "", err
	}
	return abs, filepath.ToSlash(rel), nil
}

func codegenCachePath(manifest *LoadedManifest) string {
	return filepath.Join(manifest.OpRoot(), codegenManifestName)
}

func readCodegenCache(manifest *LoadedManifest) (codegenCacheManifest, error) {
	path := codegenCachePath(manifest)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return codegenCacheManifest{}, nil
	}
	if err != nil {
		return codegenCacheManifest{}, fmt.Errorf("read %s: %w", path, err)
	}
	var cache codegenCacheManifest
	if err := json.Unmarshal(data, &cache); err != nil {
		return codegenCacheManifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cache, nil
}

func writeCodegenCache(manifest *LoadedManifest, cache codegenCacheManifest) error {
	path := codegenCachePath(manifest)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create codegen cache dir: %w", err)
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal codegen cache: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func codegenCachePlugins(plugins []resolvedCodegenPlugin) []codegenCachePlugin {
	out := make([]codegenCachePlugin, 0, len(plugins))
	for _, plugin := range plugins {
		out = append(out, codegenCachePlugin{
			Name:      plugin.Name,
			SDK:       plugin.SDK,
			Version:   plugin.Version,
			Target:    plugin.Target,
			Binary:    plugin.Binary,
			OutSubdir: plugin.OutSubdir,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func safeCodegenJoin(root, rel string) (string, error) {
	trimmed := strings.TrimSpace(rel)
	if trimmed == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(trimmed) || filepath.VolumeName(trimmed) != "" {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(trimmed))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("path escapes root")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(rootAbs, clean)
	relToRoot, err := filepath.Rel(rootAbs, joined)
	if err != nil {
		return "", err
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relToRoot) {
		return "", fmt.Errorf("path escapes root")
	}
	return joined, nil
}

func formatDeclaredCodegenPlugins(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func compareCodegenVersion(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	max := len(as)
	if len(bs) > max {
		max = len(bs)
	}
	for i := 0; i < max; i++ {
		ai, bi := 0, 0
		if i < len(as) {
			ai, _ = strconv.Atoi(codegenNumericPrefix(as[i]))
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(codegenNumericPrefix(bs[i]))
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return strings.Compare(a, b)
}

func codegenNumericPrefix(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "0"
	}
	return b.String()
}
