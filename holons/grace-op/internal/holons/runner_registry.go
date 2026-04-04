package holons

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var runnerRegistry = map[string]runner{
	RunnerGoModule: goModuleRunner{},
	RunnerCMake:    cmakeRunner{},
	RunnerCargo:    cargoRunner{},
	RunnerPython:   pythonRunner{},
	RunnerDart:     dartRunner{},
	RunnerRuby:     rubyRunner{},
	RunnerSwiftPkg: swiftPackageRunner{},
	RunnerFlutter:  flutterRunner{},
	RunnerNPM:      npmRunner{},
	RunnerGradle:   gradleRunner{},
	RunnerDotnet:   dotnetRunner{},
	RunnerQtCMake:  qtCMakeRunner{},
	RunnerRecipe:   recipeRunner{},
}

func isSupportedRunner(name string) bool {
	_, ok := runnerRegistry[strings.TrimSpace(name)]
	return ok
}

func supportedRunnerList() string {
	names := make([]string, 0, len(runnerRegistry))
	for name := range runnerRegistry {
		names = append(names, fmt.Sprintf("%q", name))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func manifestHasPrimaryArtifact(manifest *LoadedManifest) bool {
	if manifest == nil {
		return false
	}
	return strings.TrimSpace(manifest.Manifest.Artifacts.Primary) != ""
}

func requireRunnerCommands(commands ...string) error {
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			return fmt.Errorf("missing required command %q on PATH; %s", command, installHint(command))
		}
	}
	return nil
}

func hostExecutableName(name string) string {
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		return name + ".exe"
	}
	return name
}

func syncBinaryArtifact(manifest *LoadedManifest, src string) error {
	if manifest == nil || manifestHasPrimaryArtifact(manifest) {
		return nil
	}
	if strings.TrimSpace(src) == "" {
		return fmt.Errorf("build did not produce %s", manifest.BinaryName())
	}
	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		return err
	}
	return copyFile(src, manifest.BinaryPath())
}

func syncBinaryFromCandidates(manifest *LoadedManifest, candidates []string) error {
	if manifest == nil || manifestHasPrimaryArtifact(manifest) {
		return nil
	}
	if candidate := firstExistingArtifactCandidate(candidates); candidate != "" {
		return syncBinaryArtifact(manifest, candidate)
	}
	return missingBinaryFromCandidates(manifest, candidates)
}

func firstExistingArtifactCandidate(candidates []string) string {
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func missingBinaryFromCandidates(manifest *LoadedManifest, candidates []string) error {
	trimmed := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate != "" {
			trimmed = append(trimmed, workspaceRelativePath(candidate))
		}
	}
	return fmt.Errorf("build did not produce %s (searched: %s)", manifest.BinaryName(), strings.Join(trimmed, ", "))
}

func syncDotnetArtifacts(manifest *LoadedManifest, outputDir string) error {
	if manifest == nil || manifestHasPrimaryArtifact(manifest) {
		return nil
	}
	if strings.TrimSpace(outputDir) == "" {
		return fmt.Errorf("build did not produce %s", manifest.BinaryName())
	}

	binDir := filepath.Dir(manifest.BinaryPath())
	if err := os.RemoveAll(binDir); err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(outputDir, entry.Name())
		dst := filepath.Join(binDir, entry.Name())
		if err := copyArtifact(src, dst); err != nil {
			return err
		}
	}

	if _, err := os.Stat(manifest.BinaryPath()); err != nil {
		dllPath := filepath.Join(binDir, manifest.BinaryName()+".dll")
		if fileExists(dllPath) && runtime.GOOS != "windows" {
			if err := writeDotnetLauncher(manifest.BinaryPath(), manifest.BinaryName()+".dll"); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("build did not produce %s in %s", manifest.BinaryName(), workspaceRelativePath(outputDir))
		}
	}
	return nil
}

func syncGradleInstallDistSupport(manifest *LoadedManifest, launcherPath string) error {
	if manifest == nil || manifestHasPrimaryArtifact(manifest) {
		return nil
	}
	if strings.TrimSpace(launcherPath) == "" {
		return nil
	}

	installRoot := filepath.Dir(filepath.Dir(launcherPath))
	libDir := filepath.Join(installRoot, "lib")
	if !dirExists(libDir) {
		return nil
	}

	return copyArtifact(libDir, filepath.Join(manifest.HolonPackageDir(), "bin", "lib"))
}

func writeDotnetLauncher(path string, dllName string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	script := fmt.Sprintf(`#!/bin/sh
set -eu
%s
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
DOTNET_BIN=$(command -v dotnet 2>/dev/null || true)
if [ -z "$DOTNET_BIN" ]; then
  for candidate in /opt/homebrew/bin/dotnet /usr/local/bin/dotnet /opt/homebrew/share/dotnet/dotnet /usr/local/share/dotnet/dotnet /usr/share/dotnet/dotnet; do
    if [ -x "$candidate" ]; then
      DOTNET_BIN="$candidate"
      break
    fi
  done
fi
if [ -z "$DOTNET_BIN" ]; then
  echo "dotnet: not found" >&2
  exit 127
fi
exec "$DOTNET_BIN" "$SCRIPT_DIR/%s" "$@"
`, launcherPATHExports(), dllName)
	return os.WriteFile(path, []byte(script), 0o755)
}

func launcherPATHExports() string {
	return `export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin${PATH:+:$PATH}"`
}

func launcherUTF8LocaleExports() string {
	return `if [ -z "${LANG:-}" ]; then
  export LANG=en_US.UTF-8
fi
if [ -z "${LC_ALL:-}" ]; then
  export LC_ALL="$LANG"
fi`
}

func hasCMakeProject(manifest *LoadedManifest) bool {
	if manifest == nil {
		return false
	}
	info, err := os.Stat(filepath.Join(manifest.Dir, "CMakeLists.txt"))
	return err == nil && !info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func firstAvailableCommand(candidates ...string) (string, error) {
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, nil
		}
	}
	quoted := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		quoted = append(quoted, fmt.Sprintf("%q", candidate))
	}
	return "", fmt.Errorf("missing required command on PATH; expected one of %s", strings.Join(quoted, ", "))
}

func pythonInterpreter() (string, error) {
	interpreter, err := firstAvailableCommand("python3", "python")
	if err != nil {
		return "", fmt.Errorf("python runner requires python3 or python on PATH")
	}
	return interpreter, nil
}

func pythonProjectInterpreterPath(manifest *LoadedManifest) string {
	if manifest == nil {
		return ""
	}
	for _, rel := range []string{
		filepath.Join(".venv", "bin", "python"),
		filepath.Join(".venv", "bin", "python3"),
		filepath.Join("venv", "bin", "python"),
		filepath.Join("venv", "bin", "python3"),
	} {
		candidate := filepath.Join(manifest.Dir, rel)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return ""
}

func pythonInterpreterForManifest(manifest *LoadedManifest) (string, error) {
	if local := pythonProjectInterpreterPath(manifest); local != "" {
		return local, nil
	}
	return pythonInterpreter()
}

func pythonInterpreterPath() (string, error) {
	for _, candidate := range []string{"python3", "python"} {
		resolved, err := exec.LookPath(candidate)
		if err == nil {
			cmd := exec.Command(resolved, "-c", "import sys; print(sys.executable)")
			output, outputErr := cmd.Output()
			if outputErr == nil {
				actual := strings.TrimSpace(string(output))
				if actual != "" {
					return actual, nil
				}
			}
			return resolved, nil
		}
	}
	return "", fmt.Errorf("python runner requires python3 or python on PATH")
}

func pythonInterpreterPathForManifest(manifest *LoadedManifest) (string, error) {
	if local := pythonProjectInterpreterPath(manifest); local != "" {
		return local, nil
	}
	return pythonInterpreterPath()
}

func pythonBuildArgs(manifest *LoadedManifest) ([]string, bool, error) {
	interpreter, err := pythonInterpreterForManifest(manifest)
	if err != nil {
		return nil, false, err
	}
	if !fileExists(filepath.Join(manifest.Dir, "requirements.txt")) {
		return nil, false, nil
	}
	return []string{interpreter, "-m", "pip", "install", "-r", "requirements.txt"}, true, nil
}

func pythonEntrypoint(manifest *LoadedManifest, searchDir string) (string, error) {
	// Prefer the explicit entrypoint declared in build.main.
	if rel := strings.TrimPrefix(strings.TrimSpace(manifest.Manifest.Build.Main), "./"); rel != "" {
		if fileExists(filepath.Join(searchDir, filepath.FromSlash(rel))) {
			return rel, nil
		}
	}
	for _, rel := range []string{"bin/main.py", "main.py", "app/main.py"} {
		if fileExists(filepath.Join(searchDir, filepath.FromSlash(rel))) {
			return rel, nil
		}
	}
	return "", fmt.Errorf("python runner requires bin/main.py, main.py, or app/main.py")
}

func pythonTestArgs(manifest *LoadedManifest) ([]string, error) {
	interpreter, err := pythonInterpreterForManifest(manifest)
	if err != nil {
		return nil, err
	}
	if dirExists(filepath.Join(manifest.Dir, "tests")) {
		return []string{interpreter, "-m", "unittest", "discover"}, nil
	}
	if _, err := exec.LookPath("pytest"); err == nil {
		return []string{"pytest"}, nil
	}
	return nil, fmt.Errorf("python runner requires tests/ or pytest on PATH")
}

func dartEntrypoint(manifest *LoadedManifest) (string, error) {
	// Prefer the explicit entrypoint declared in build.main.
	if rel := strings.TrimPrefix(strings.TrimSpace(manifest.Manifest.Build.Main), "./"); rel != "" {
		if fileExists(filepath.Join(manifest.Dir, filepath.FromSlash(rel))) {
			return rel, nil
		}
	}
	for _, rel := range []string{"bin/main.dart", "lib/main.dart"} {
		if fileExists(filepath.Join(manifest.Dir, filepath.FromSlash(rel))) {
			return rel, nil
		}
	}
	return "", fmt.Errorf("dart runner requires bin/main.dart or lib/main.dart")
}

func executableFile(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func rubyToolchainPaths() (string, string, error) {
	type candidate struct {
		rubyPath   string
		bundlePath string
	}

	addCandidate := func(dst *[]candidate, rubyPath string, bundlePath string) {
		rubyPath = strings.TrimSpace(rubyPath)
		bundlePath = strings.TrimSpace(bundlePath)
		if rubyPath == "" {
			return
		}
		if !executableFile(rubyPath) {
			return
		}
		if bundlePath == "" {
			bundlePath = filepath.Join(filepath.Dir(rubyPath), "bundle")
		}
		if !executableFile(bundlePath) {
			return
		}
		*dst = append(*dst, candidate{rubyPath: rubyPath, bundlePath: bundlePath})
	}

	var candidates []candidate
	if rubyPath, err := exec.LookPath("ruby"); err == nil && rubyPath != "" && rubyPath != "/usr/bin/ruby" {
		bundlePath, _ := exec.LookPath("bundle")
		addCandidate(&candidates, rubyPath, bundlePath)
	}

	if homeDir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(homeDir) != "" {
		for _, base := range []string{
			filepath.Join(homeDir, ".rbenv", "shims"),
			filepath.Join(homeDir, ".asdf", "shims"),
			filepath.Join(homeDir, ".mise", "shims"),
		} {
			addCandidate(&candidates, filepath.Join(base, "ruby"), filepath.Join(base, "bundle"))
		}
	}

	for _, base := range []string{
		"/opt/homebrew/opt/ruby/bin",
		"/usr/local/opt/ruby/bin",
	} {
		addCandidate(&candidates, filepath.Join(base, "ruby"), filepath.Join(base, "bundle"))
	}

	for _, pattern := range []string{
		"/opt/homebrew/Cellar/ruby/*/bin/ruby",
		"/usr/local/Cellar/ruby/*/bin/ruby",
		"/opt/homebrew/Library/Homebrew/vendor/portable-ruby/*/bin/ruby",
		"/usr/local/Homebrew/Library/Homebrew/vendor/portable-ruby/*/bin/ruby",
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		for _, rubyPath := range matches {
			addCandidate(&candidates, rubyPath, filepath.Join(filepath.Dir(rubyPath), "bundle"))
		}
	}

	if rubyPath, err := exec.LookPath("ruby"); err == nil {
		bundlePath, _ := exec.LookPath("bundle")
		addCandidate(&candidates, rubyPath, bundlePath)
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		key := candidate.rubyPath + "\x00" + candidate.bundlePath
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		return candidate.rubyPath, candidate.bundlePath, nil
	}

	return "", "", fmt.Errorf("ruby runner requires ruby and bundle (PATH or a local toolchain)")
}

func rubyGemPath(rubyPath string) string {
	candidate := filepath.Join(filepath.Dir(rubyPath), "gem")
	if executableFile(candidate) {
		return candidate
	}
	return "gem"
}

func rubyBase64LibPath(searchDir string) string {
	if searchDir == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(searchDir, ".op", "base64", "gems", "base64-*", "lib"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}

func sharedCacheBaseDir() string {
	if configured := strings.TrimSpace(os.Getenv("GRACE_OP_SHARED_CACHE_DIR")); configured != "" {
		return configured
	}
	return os.TempDir()
}

func rubySharedCacheRoot(manifest *LoadedManifest) string {
	baseDir := sharedCacheBaseDir()
	name := "ruby"
	if manifest != nil {
		if dir := strings.TrimSpace(manifest.Dir); dir != "" {
			name = filepath.Base(dir)
			hasher := fnv.New64a()
			_, _ = hasher.Write([]byte(filepath.Clean(dir)))
			return filepath.Join(baseDir, "grace-op-ruby-cache", fmt.Sprintf("%s-%x", name, hasher.Sum64()))
		}
		if binary := strings.TrimSpace(manifest.BinaryName()); binary != "" {
			name = binary
		}
	}

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(name))
	return filepath.Join(baseDir, "grace-op-ruby-cache", fmt.Sprintf("%s-%x", name, hasher.Sum64()))
}

func rubySharedBundleDir(manifest *LoadedManifest) string {
	return filepath.Join(rubySharedCacheRoot(manifest), "bundle")
}

func rubySharedBase64Dir(manifest *LoadedManifest) string {
	return filepath.Join(rubySharedCacheRoot(manifest), "base64")
}

func prepareRubySharedCache(isolatedDir string, manifest *LoadedManifest) error {
	if err := os.MkdirAll(filepath.Join(isolatedDir, ".op"), 0o755); err != nil {
		return err
	}

	links := []struct {
		linkPath   string
		targetPath string
	}{
		{
			linkPath:   filepath.Join(isolatedDir, ".op", "bundle"),
			targetPath: rubySharedBundleDir(manifest),
		},
		{
			linkPath:   filepath.Join(isolatedDir, ".op", "base64"),
			targetPath: rubySharedBase64Dir(manifest),
		},
	}

	for _, link := range links {
		if err := os.MkdirAll(filepath.Dir(link.targetPath), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(link.targetPath, 0o755); err != nil {
			return err
		}
		if err := os.RemoveAll(link.linkPath); err != nil {
			return err
		}
		if err := os.Symlink(link.targetPath, link.linkPath); err != nil {
			return err
		}
	}

	return nil
}

func prepareRubyIsolatedWorkspace(isolatedDir string, manifest *LoadedManifest) error {
	// Re-anchor relative SDK dependencies to absolute paths to survive inner execution.
	supportPath := filepath.Join(isolatedDir, "support.rb")
	if b, err := os.ReadFile(supportPath); err == nil {
		s := strings.ReplaceAll(string(b), "\"../../../sdk/ruby-holons/lib\"", fmt.Sprintf("\"%s/../../../sdk/ruby-holons/lib\"", filepath.ToSlash(manifest.Dir)))
		if err := os.WriteFile(supportPath, []byte(s), 0o644); err != nil {
			return err
		}
	}
	if err := prepareRubySharedCache(isolatedDir, manifest); err != nil {
		return fmt.Errorf("prepare ruby cache: %w", err)
	}
	return nil
}

func rubySetupCommands(bundlePath, gemPath string) [][]string {
	return [][]string{
		{"env", "BUNDLE_FORCE_RUBY_PLATFORM=true", bundlePath, "lock", "--add-platform", "arm64-darwin"},
		{bundlePath, "config", "set", "--local", "path", ".op/bundle"},
		{"env", "BUNDLE_FORCE_RUBY_PLATFORM=true", bundlePath, "install"},
		{gemPath, "install", "base64", "--install-dir", ".op/base64", "--no-document"},
	}
}

func rubyTestArgs(manifest *LoadedManifest) ([]string, error) {
	_, bundlePath, err := rubyToolchainPaths()
	if err != nil {
		return nil, err
	}
	if dirExists(filepath.Join(manifest.Dir, "spec")) {
		return []string{bundlePath, "exec", "rspec"}, nil
	}
	if fileExists(filepath.Join(manifest.Dir, "Rakefile")) {
		return []string{bundlePath, "exec", "rake", "test"}, nil
	}
	return nil, fmt.Errorf("ruby runner requires spec/ or Rakefile")
}

func rubyEntrypoint(manifest *LoadedManifest) (string, error) {
	searchDir := ""
	if manifest != nil {
		searchDir = manifest.Dir
	}
	return rubyEntrypointInDir(manifest, searchDir)
}

func rubyEntrypointInDir(manifest *LoadedManifest, searchDir string) (string, error) {
	// Prefer the explicit entrypoint declared in build.main.
	if rel := strings.TrimPrefix(strings.TrimSpace(manifest.Manifest.Build.Main), "./"); rel != "" {
		if fileExists(filepath.Join(searchDir, filepath.FromSlash(rel))) {
			return rel, nil
		}
	}
	for _, rel := range []string{"bin/main.rb", "main.rb", "app/main.rb"} {
		if fileExists(filepath.Join(searchDir, filepath.FromSlash(rel))) {
			return rel, nil
		}
	}
	return "", fmt.Errorf("ruby runner requires bin/main.rb, main.rb, or app/main.rb")
}

func removeSelectedPaths(root string, relPaths ...string) error {
	for _, relPath := range relPaths {
		if err := os.RemoveAll(filepath.Join(root, relPath)); err != nil {
			return err
		}
	}
	return nil
}

func removeNamedDirs(root string, names ...string) error {
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[name] = struct{}{}
	}

	var matches []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, ok := nameSet[d.Name()]; ok {
			matches = append(matches, path)
			return filepath.SkipDir
		}
		return nil
	}); err != nil {
		return err
	}

	sort.Slice(matches, func(i, j int) bool { return len(matches[i]) > len(matches[j]) })
	for _, match := range matches {
		if err := os.RemoveAll(match); err != nil {
			return err
		}
	}
	return nil
}

type cargoRunner struct{}

func (cargoRunner) check(manifest *LoadedManifest, _ BuildContext) error {
	if err := requireRunnerCommands("cargo", "rustc"); err != nil {
		return err
	}
	if hasCMakeProject(manifest) {
		return requireRunnerCommands("cmake")
	}
	return nil
}

func (cargoRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if hasCMakeProject(manifest) {
		return cmakeRunner{}.build(manifest, ctx, report)
	}
	if err := ensureHostBuildTarget(RunnerCargo, ctx); err != nil {
		return err
	}

	targetDir := filepath.Join(manifest.OpRoot(), "build", "cargo")
	args := []string{"cargo", "build", "--target-dir", targetDir}
	if normalizeBuildMode(ctx.Mode) != buildModeDebug {
		args = append(args, "--release")
	}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if ctx.DryRun {
		return nil
	}
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	source := filepath.Join(targetDir, cargoModeDir(ctx.Mode), hostExecutableName(manifest.BinaryName()))
	if err := syncBinaryArtifact(manifest, source); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "cargo build complete")
	return nil
}

func (cargoRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	args := []string{"cargo", "test", "--target-dir", filepath.Join(manifest.OpRoot(), "build", "cargo")}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "cargo test passed")
	return nil
}

func (cargoRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

func cargoModeDir(mode string) string {
	if normalizeBuildMode(mode) == buildModeDebug {
		return "debug"
	}
	return "release"
}

type pythonRunner struct{}

func (pythonRunner) check(_ *LoadedManifest, _ BuildContext) error {
	_, err := pythonInterpreter()
	return err
}

func (pythonRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerPython, ctx); err != nil {
		return err
	}

	isolatedDir := filepath.Join(manifest.OpRoot(), "build", "python")
	if !ctx.DryRun {
		_ = os.RemoveAll(isolatedDir)
		if err := copyWorkspaceIsolated(manifest.Dir, isolatedDir); err != nil {
			return fmt.Errorf("isolate python workspace: %w", err)
		}

		// Re-anchor relative SDK dependencies to absolute paths to survive inner execution
		supportPath := filepath.Join(isolatedDir, "support.py")
		if b, err := os.ReadFile(supportPath); err == nil {
			s := strings.ReplaceAll(string(b), "ROOT.parent.parent.parent", fmt.Sprintf("Path(%q).parent.parent.parent", filepath.ToSlash(manifest.Dir)))
			_ = os.WriteFile(supportPath, []byte(s), 0o644)
		}
	}

	args, ok, err := pythonBuildArgs(manifest)
	if err != nil {
		return err
	}
	if !ok {
		report.Notes = append(report.Notes, "no requirements.txt; skipping dependency install")
	} else {
		report.Commands = append(report.Commands, commandString(args))
		ctx.Progress.Step(commandString(args))
		if !ctx.DryRun {
			if output, err := runCommand(isolatedDir, args); err != nil {
				return fmt.Errorf("%s\n%s", err, output)
			}
			report.Notes = append(report.Notes, "python dependencies installed")
		}
	}

	if manifestHasPrimaryArtifact(manifest) || ctx.DryRun {
		return nil
	}

	entrypoint, err := pythonEntrypoint(manifest, isolatedDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		return err
	}
	wrapper := fmt.Sprintf(
		"#!/bin/sh\nset -eu\nexec %q %q \"$@\"\n",
		argsOrDefaultPythonPathForManifest(manifest),
		filepath.Join(isolatedDir, filepath.FromSlash(entrypoint)),
	)
	if err := os.WriteFile(manifest.BinaryPath(), []byte(wrapper), 0o755); err != nil {
		return err
	}
	report.Notes = append(report.Notes, fmt.Sprintf("python launcher prepared for %s", entrypoint))
	return nil
}

func argsOrDefaultPython() string {
	interpreter, err := pythonInterpreter()
	if err != nil {
		return "python3"
	}
	return interpreter
}

func argsOrDefaultPythonPath() string {
	interpreter, err := pythonInterpreterPath()
	if err != nil {
		return argsOrDefaultPython()
	}
	return interpreter
}

func argsOrDefaultPythonPathForManifest(manifest *LoadedManifest) string {
	interpreter, err := pythonInterpreterPathForManifest(manifest)
	if err != nil {
		return argsOrDefaultPython()
	}
	return interpreter
}

func nodeInterpreterPath() (string, error) {
	resolved, err := exec.LookPath("node")
	if err != nil {
		return "", fmt.Errorf("npm runner requires node on PATH")
	}

	cmd := exec.Command(resolved, "-p", "process.execPath")
	output, outputErr := cmd.Output()
	if outputErr == nil {
		actual := strings.TrimSpace(string(output))
		if actual != "" {
			return actual, nil
		}
	}
	return resolved, nil
}

func argsOrDefaultNodePath() string {
	interpreter, err := nodeInterpreterPath()
	if err != nil {
		return "node"
	}
	return interpreter
}

func (pythonRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerPython, ctx); err != nil {
		return err
	}

	args, err := pythonTestArgs(manifest)
	if err != nil {
		return err
	}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "python tests passed")
	return nil
}

func (pythonRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	if err := removeNamedDirs(manifest.Dir, "__pycache__"); err != nil {
		return err
	}
	if err := removeSelectedPaths(manifest.Dir, ".pytest_cache", "build", "dist"); err != nil {
		return err
	}
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed Python caches, build/, dist/, and .op/")
	return nil
}

type dartRunner struct{}

func (dartRunner) check(manifest *LoadedManifest, _ BuildContext) error {
	if err := requireRunnerCommands("dart"); err != nil {
		return err
	}
	if !fileExists(filepath.Join(manifest.Dir, "pubspec.yaml")) {
		return fmt.Errorf("dart runner requires pubspec.yaml")
	}
	_, err := dartEntrypoint(manifest)
	return err
}

func (dartRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerDart, ctx); err != nil {
		return err
	}

	entrypoint, err := dartEntrypoint(manifest)
	if err != nil {
		return err
	}
	outputPath := manifest.BinaryPath()
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("dart runner requires an artifact output path")
	}

	commands := [][]string{
		{"dart", "pub", "get"},
		{"dart", "compile", "exe", filepath.FromSlash(entrypoint), "-o", outputPath},
	}
	for _, args := range commands {
		report.Commands = append(report.Commands, commandString(args))
		ctx.Progress.Step(commandString(args))
	}
	if ctx.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	for _, args := range commands {
		if output, err := runCommand(manifest.Dir, args); err != nil {
			return fmt.Errorf("%s\n%s", err, output)
		}
	}
	report.Notes = append(report.Notes, "dart build complete")
	return nil
}

func (dartRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerDart, ctx); err != nil {
		return err
	}

	args := []string{"dart", "test"}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "dart test passed")
	return nil
}

func (dartRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	if err := removeSelectedPaths(manifest.Dir, "build", ".dart_tool"); err != nil {
		return err
	}
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed build/, .dart_tool/, and .op/")
	return nil
}

type rubyRunner struct{}

func (rubyRunner) check(manifest *LoadedManifest, _ BuildContext) error {
	if !fileExists(filepath.Join(manifest.Dir, "Gemfile")) {
		return fmt.Errorf("ruby runner requires Gemfile")
	}
	_, _, err := rubyToolchainPaths()
	return err
}

func (rubyRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerRuby, ctx); err != nil {
		return err
	}
	rubyPath, bundlePath, err := rubyToolchainPaths()
	if err != nil {
		return err
	}
	gemPath := rubyGemPath(rubyPath)

	isolatedDir := filepath.Join(manifest.OpRoot(), "build", "ruby")
	if !ctx.DryRun {
		_ = os.RemoveAll(isolatedDir)
		if err := copyWorkspaceIsolated(manifest.Dir, isolatedDir); err != nil {
			return fmt.Errorf("isolate ruby workspace: %w", err)
		}
		if err := prepareRubyIsolatedWorkspace(isolatedDir, manifest); err != nil {
			return err
		}
	}

	_ = os.RemoveAll(filepath.Join(isolatedDir, "vendor", "bundle"))

	commands := rubySetupCommands(bundlePath, gemPath)
	for _, args := range commands {
		report.Commands = append(report.Commands, commandString(args))
		ctx.Progress.Step(commandString(args))
	}
	if ctx.DryRun {
		return nil
	}
	for _, args := range commands {
		if output, err := runCommand(isolatedDir, args); err != nil {
			return fmt.Errorf("%s\n%s", err, output)
		}
	}
	if manifestHasPrimaryArtifact(manifest) {
		report.Notes = append(report.Notes, "bundle install complete")
		return nil
	}

	entrypoint, err := rubyEntrypointInDir(manifest, isolatedDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		return err
	}
	rubyLibExport := ""
	if base64Lib := rubyBase64LibPath(isolatedDir); base64Lib != "" {
		rubyLibExport = fmt.Sprintf("export RUBYLIB=%q${RUBYLIB:+:$RUBYLIB}\n", base64Lib)
	}
	sourceEntrypoint := filepath.Join(manifest.Dir, filepath.FromSlash(entrypoint))
	wrapper := fmt.Sprintf(
		"#!/bin/sh\nset -eu\n%s\n%s\n_OP_BASE=%q\n_OP_SOURCE_ENTRYPOINT=%q\nexport BUNDLE_GEMFILE=\"$_OP_BASE/Gemfile\"\nexport BUNDLE_PATH=\"$_OP_BASE/.op/bundle\"\nexport BUNDLE_DISABLE_SHARED_GEMS=true\nexport BUNDLE_FORCE_RUBY_PLATFORM=true\n%s\nexec %q exec %q \"$_OP_BASE/%s\" \"$@\"\n",
		launcherPATHExports(),
		launcherUTF8LocaleExports(),
		isolatedDir,
		sourceEntrypoint,
		rubyLibExport,
		bundlePath,
		rubyPath,
		filepath.ToSlash(entrypoint),
	)
	if err := os.WriteFile(manifest.BinaryPath(), []byte(wrapper), 0o755); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "bundle install complete")
	return nil
}

func (rubyRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerRuby, ctx); err != nil {
		return err
	}
	rubyPath, bundlePath, err := rubyToolchainPaths()
	if err != nil {
		return err
	}
	gemPath := rubyGemPath(rubyPath)

	isolatedDir := filepath.Join(manifest.OpRoot(), "test", "ruby")
	if !ctx.DryRun {
		_ = os.RemoveAll(isolatedDir)
		if err := copyWorkspaceIsolated(manifest.Dir, isolatedDir); err != nil {
			return fmt.Errorf("isolate ruby workspace: %w", err)
		}
		if err := prepareRubyIsolatedWorkspace(isolatedDir, manifest); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(filepath.Join(isolatedDir, "vendor", "bundle"))

	args, err := rubyTestArgs(manifest)
	if err != nil {
		return err
	}

	commands := append(rubySetupCommands(bundlePath, gemPath), args)
	for _, cmdArgs := range commands {
		report.Commands = append(report.Commands, commandString(cmdArgs))
		ctx.Progress.Step(commandString(cmdArgs))
	}
	if ctx.DryRun {
		return nil
	}
	for _, cmdArgs := range commands {
		if output, err := runCommand(isolatedDir, cmdArgs); err != nil {
			return fmt.Errorf("%s\n%s", err, output)
		}
	}
	report.Notes = append(report.Notes, "ruby tests passed")
	return nil
}

func (rubyRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	if err := removeSelectedPaths(manifest.Dir, "log", "tmp", ".bundle", filepath.Join("vendor", "bundle")); err != nil {
		return err
	}
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed log/, tmp/, .bundle/, vendor/bundle/, and .op/")
	return nil
}

type swiftPackageRunner struct{}

var swiftPackageRunCommand = runCommand

func (swiftPackageRunner) check(manifest *LoadedManifest, _ BuildContext) error {
	if err := requireRunnerCommands("swift", "xcodebuild"); err != nil {
		return err
	}
	if hasSwiftPackage(manifest) {
		return nil
	}
	if _, _, ok := detectXcodeContainer(manifest); ok {
		return nil
	}
	return fmt.Errorf("swift-package runner requires Package.swift, .xcodeproj, or .xcworkspace")
}

func (swiftPackageRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerSwiftPkg, ctx); err != nil {
		return err
	}

	if hasSwiftPackage(manifest) {
		buildPath := swiftPackageBuildPath(manifest)
		args := []string{"swift", "build", "--build-path", buildPath, "-c", swiftBuildMode(ctx.Mode)}
		report.Commands = append(report.Commands, commandString(args))
		ctx.Progress.Step(commandString(args))
		if ctx.DryRun {
			return nil
		}
		output, err := swiftPackageRunCommand(manifest.Dir, args)
		if err != nil {
			recoveryNote := "swift build cache reset after failed build"
			if isSwiftBuildDescriptionCorruption(output) {
				cleanArgs := []string{"swift", "package", "clean", "--build-path", buildPath}
				report.Commands = append(report.Commands, commandString(cleanArgs), commandString(args))
				ctx.Progress.Step(commandString(cleanArgs))
				if cleanOutput, cleanErr := swiftPackageRunCommand(manifest.Dir, cleanArgs); cleanErr != nil {
					return fmt.Errorf("%s\n%s\n%s\n%s", err, output, cleanErr, cleanOutput)
				}
				recoveryNote = "swift build cache reset after unknown build description"
			} else if resetErr := os.RemoveAll(buildPath); resetErr != nil {
				return fmt.Errorf("%s\n%s\nreset swift build path %s: %v", err, output, buildPath, resetErr)
			}

			ctx.Progress.Step(commandString(args))
			if output, err = swiftPackageRunCommand(manifest.Dir, args); err != nil {
				return fmt.Errorf("%s\n%s", err, output)
			}
			report.Notes = append(report.Notes, recoveryNote)
		}
		source := swiftPackageArtifactSource(manifest, buildPath, ctx.Mode)
		if err := syncBinaryArtifact(manifest, source); err != nil {
			return err
		}
		report.Notes = append(report.Notes, "swift build complete")
		return nil
	}

	flag, container, _ := detectXcodeContainer(manifest)
	symroot := filepath.Join(manifest.OpRoot(), "build", "xcode")
	args := []string{
		"xcodebuild",
		flag, container,
		"-scheme", manifest.BinaryName(),
		"-configuration", cmakeBuildConfig(ctx.Mode),
		"SYMROOT=" + symroot,
		"build",
	}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if ctx.DryRun {
		return nil
	}
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	source := filepath.Join(symroot, "Build", "Products", cmakeBuildConfig(ctx.Mode), hostExecutableName(manifest.BinaryName()))
	if err := syncBinaryArtifact(manifest, source); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "xcode build complete")
	return nil
}

func (swiftPackageRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if hasSwiftPackage(manifest) {
		args := []string{"swift", "test", "--build-path", swiftPackageBuildPath(manifest), "-c", swiftBuildMode(ctx.Mode)}
		report.Commands = append(report.Commands, commandString(args))
		ctx.Progress.Step(commandString(args))
		if output, err := runCommand(manifest.Dir, args); err != nil {
			return fmt.Errorf("%s\n%s", err, output)
		}
		report.Notes = append(report.Notes, "swift test passed")
		return nil
	}
	return fmt.Errorf("swift-package test is only supported for Package.swift projects")
}

func (swiftPackageRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	if hasSwiftPackage(manifest) {
		args := []string{"swift", "package", "clean"}
		report.Commands = append(report.Commands, commandString(args))
		if _, err := exec.LookPath("swift"); err == nil {
			if output, cmdErr := runCommand(manifest.Dir, args); cmdErr != nil {
				return fmt.Errorf("%s\n%s", cmdErr, output)
			}
		}
	}
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

func hasSwiftPackage(manifest *LoadedManifest) bool {
	info, err := os.Stat(filepath.Join(manifest.Dir, "Package.swift"))
	return err == nil && !info.IsDir()
}

func detectXcodeContainer(manifest *LoadedManifest) (string, string, bool) {
	workspaceMatches, _ := filepath.Glob(filepath.Join(manifest.Dir, "*.xcworkspace"))
	if len(workspaceMatches) > 0 {
		return "-workspace", filepath.Base(workspaceMatches[0]), true
	}
	projectMatches, _ := filepath.Glob(filepath.Join(manifest.Dir, "*.xcodeproj"))
	if len(projectMatches) > 0 {
		return "-project", filepath.Base(projectMatches[0]), true
	}
	return "", "", false
}

func swiftBuildMode(mode string) string {
	if normalizeBuildMode(mode) == buildModeDebug {
		return "debug"
	}
	return "release"
}

func swiftPackageBuildPath(manifest *LoadedManifest) string {
	baseDir := sharedCacheBaseDir()
	name := "swift-package"
	if manifest != nil {
		if dir := strings.TrimSpace(manifest.Dir); dir != "" {
			name = filepath.Base(dir)
			hasher := fnv.New64a()
			_, _ = hasher.Write([]byte(filepath.Clean(dir)))
			return filepath.Join(baseDir, "grace-op-swift-cache", fmt.Sprintf("%s-%x", name, hasher.Sum64()))
		}
		if binary := strings.TrimSpace(manifest.BinaryName()); binary != "" {
			name = binary
		}
	}

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(name))
	return filepath.Join(baseDir, "grace-op-swift-cache", fmt.Sprintf("%s-%x", name, hasher.Sum64()))
}

func swiftPackageArtifactSource(manifest *LoadedManifest, buildPath, mode string) string {
	executable := hostExecutableName(manifest.BinaryName())
	sharedPath := filepath.Join(buildPath, swiftBuildMode(mode), executable)
	if _, err := os.Stat(sharedPath); err == nil {
		return sharedPath
	}
	return filepath.Join(manifest.OpRoot(), "build", "swift", swiftBuildMode(mode), executable)
}

func isSwiftBuildDescriptionCorruption(output string) bool {
	return strings.Contains(strings.ToLower(output), "unknown build description")
}

type flutterRunner struct{}

func (flutterRunner) check(_ *LoadedManifest, _ BuildContext) error {
	return requireRunnerCommands("flutter", "dart")
}

func (flutterRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	args, err := flutterBuildArgs(ctx)
	if err != nil {
		return err
	}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if ctx.DryRun {
		return nil
	}
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "flutter build complete")
	return nil
}

func (flutterRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	args := []string{"flutter", "test"}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "flutter test passed")
	return nil
}

func (flutterRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	args := []string{"flutter", "clean"}
	report.Commands = append(report.Commands, commandString(args))
	if _, err := exec.LookPath("flutter"); err == nil {
		if output, cmdErr := runCommand(manifest.Dir, args); cmdErr != nil {
			return fmt.Errorf("%s\n%s", cmdErr, output)
		}
	}
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

func flutterBuildArgs(ctx BuildContext) ([]string, error) {
	target := normalizePlatformName(ctx.Target)
	modeFlag := "--debug"
	switch normalizeBuildMode(ctx.Mode) {
	case buildModeRelease:
		modeFlag = "--release"
	case buildModeProfile:
		modeFlag = "--profile"
	}
	switch target {
	case "macos", "linux", "windows":
		return []string{"flutter", "build", target, modeFlag}, nil
	case "ios":
		return []string{"flutter", "build", "ios", modeFlag, "--no-codesign"}, nil
	case "android":
		return []string{"flutter", "build", "apk", modeFlag}, nil
	default:
		return nil, fmt.Errorf("flutter runner does not support target %q", ctx.Target)
	}
}

func prepareNPMIsolatedWorkspace(isolatedDir string, manifest *LoadedManifest) error {
	// Re-anchor relative file dependencies to absolute paths to survive inner execution.
	for _, name := range []string{"package.json", "package-lock.json"} {
		path := filepath.Join(isolatedDir, name)
		if b, err := os.ReadFile(path); err == nil {
			s := strings.ReplaceAll(string(b), "\"file:.", fmt.Sprintf("\"file:%s/.", filepath.ToSlash(manifest.Dir)))
			if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

type npmRunner struct{}

func (npmRunner) check(manifest *LoadedManifest, _ BuildContext) error {
	if err := requireRunnerCommands("node", "npm"); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(manifest.Dir, "package.json")); err != nil {
		return fmt.Errorf("npm runner requires package.json")
	}
	return nil
}

func (npmRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	isolatedDir := filepath.Join(manifest.OpRoot(), "build", "npm")
	if !ctx.DryRun {
		_ = os.RemoveAll(isolatedDir)
		if err := copyWorkspaceIsolated(manifest.Dir, isolatedDir); err != nil {
			return fmt.Errorf("isolate npm workspace: %w", err)
		}
		if err := prepareNPMIsolatedWorkspace(isolatedDir, manifest); err != nil {
			return err
		}
	}

	var commands [][]string
	commands = append(commands, []string{"npm", "ci"})
	if npmHasBuildScript(manifest) {
		commands = append(commands, []string{"npm", "run", "build"})
	}
	for _, args := range commands {
		report.Commands = append(report.Commands, commandString(args))
		ctx.Progress.Step(commandString(args))
	}
	if ctx.DryRun {
		return nil
	}
	for _, args := range commands {
		if output, err := runCommand(isolatedDir, args); err != nil {
			return fmt.Errorf("%s\n%s", err, output)
		}
	}
	if manifestHasPrimaryArtifact(manifest) {
		report.Notes = append(report.Notes, "npm build complete")
		return nil
	}

	candidates := npmArtifactCandidates(manifest, isolatedDir)
	candidates = append(candidates, npmArtifactCandidates(manifest, manifest.Dir)...)
	candidate := firstExistingArtifactCandidate(candidates)
	if candidate == "" {
		return missingBinaryFromCandidates(manifest, candidates)
	}
	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		return err
	}
	descriptorSeed := ""
	if descriptorProto := nodeDescriptorProtoSource(isolatedDir); descriptorProto != "" {
		descriptorSeed += fmt.Sprintf(
			"descriptor_src=%q\ndescriptor_dst=\"$_OP_BASE/protos/holons/v1/google/protobuf/descriptor.proto\"\nif [ -f \"$descriptor_src\" ] && [ ! -f \"$descriptor_dst\" ]; then\n  mkdir -p \"$(dirname \"$descriptor_dst\")\"\n  cp \"$descriptor_src\" \"$descriptor_dst\"\nfi\n",
			descriptorProto,
		)
	}
	wrapper := fmt.Sprintf(
		"#!/bin/sh\nset -eu\n_OP_BASE=%q\n%scd \"$_OP_BASE\"\nexec %q %q \"$@\"\n",
		isolatedDir,
		descriptorSeed,
		argsOrDefaultNodePath(),
		candidate,
	)
	if err := os.WriteFile(manifest.BinaryPath(), []byte(wrapper), 0o755); err != nil {
		return err
	}

	report.Notes = append(report.Notes, "npm build complete")
	report.Notes = append(report.Notes, fmt.Sprintf("npm launcher prepared for %s", workspaceRelativePath(candidate)))
	return nil
}

func (npmRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	isolatedDir := filepath.Join(manifest.OpRoot(), "test", "npm")
	if !ctx.DryRun {
		_ = os.RemoveAll(isolatedDir)
		if err := copyWorkspaceIsolated(manifest.Dir, isolatedDir); err != nil {
			return fmt.Errorf("isolate npm workspace: %w", err)
		}
		if err := prepareNPMIsolatedWorkspace(isolatedDir, manifest); err != nil {
			return err
		}
	}

	commands := [][]string{
		{"npm", "ci"},
		{"npm", "test"},
	}
	for _, args := range commands {
		report.Commands = append(report.Commands, commandString(args))
		ctx.Progress.Step(commandString(args))
	}
	if ctx.DryRun {
		return nil
	}
	for _, args := range commands {
		if output, err := runCommand(isolatedDir, args); err != nil {
			return fmt.Errorf("%s\n%s", err, output)
		}
	}
	report.Notes = append(report.Notes, "npm test passed")
	return nil
}

func (npmRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	for _, dir := range []string{"node_modules", "dist", "build"} {
		if err := os.RemoveAll(filepath.Join(manifest.Dir, dir)); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed node_modules/, dist/, build/, and .op/")
	return nil
}

func npmArtifactCandidates(manifest *LoadedManifest, searchDir string) []string {
	name := manifest.BinaryName()
	var candidates []string
	// Prefer the explicit entrypoint declared in build.main.
	if rel := strings.TrimPrefix(strings.TrimSpace(manifest.Manifest.Build.Main), "./"); rel != "" {
		if fileExists(filepath.Join(searchDir, filepath.FromSlash(rel))) {
			candidates = append(candidates, filepath.Join(searchDir, filepath.FromSlash(rel)))
		}
	}
	candidates = append(candidates,
		filepath.Join(searchDir, "dist", name),
		filepath.Join(searchDir, "dist", name+".js"),
		filepath.Join(searchDir, "build", name),
		filepath.Join(searchDir, "build", name+".js"),
	)
	return candidates
}

func nodeDescriptorProtoSource(searchDir string) string {
	if searchDir == "" {
		return ""
	}
	for _, candidate := range []string{
		filepath.Join(searchDir, "node_modules", "protobufjs", "google", "protobuf", "descriptor.proto"),
		filepath.Join(searchDir, "node_modules", "grpc-tools", "bin", "google", "protobuf", "descriptor.proto"),
	} {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

// npmHasBuildScript reports whether the holon's package.json declares a "build" script.
func npmHasBuildScript(manifest *LoadedManifest) bool {
	data, err := os.ReadFile(filepath.Join(manifest.Dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	_, ok := pkg.Scripts["build"]
	return ok
}

type gradleRunner struct{}

func (gradleRunner) check(manifest *LoadedManifest, _ BuildContext) error {
	if err := requireRunnerCommands("java"); err != nil {
		return err
	}
	if _, err := gradleInvoker(manifest); err != nil {
		return err
	}
	return nil
}

func (gradleRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	invoker, err := gradleInvoker(manifest)
	if err != nil {
		return err
	}
	args := append(invoker, "build", "installDist")
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if ctx.DryRun {
		return nil
	}
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	launcherPath := firstExistingArtifactCandidate(gradleArtifactCandidates(manifest))
	if launcherPath == "" {
		return missingBinaryFromCandidates(manifest, gradleArtifactCandidates(manifest))
	}
	if err := syncBinaryArtifact(manifest, launcherPath); err != nil {
		return err
	}
	if err := syncGradleInstallDistSupport(manifest, launcherPath); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "gradle build complete")
	if dirExists(filepath.Join(filepath.Dir(filepath.Dir(launcherPath)), "lib")) {
		report.Notes = append(report.Notes, "gradle runtime libraries copied into package")
	}
	return nil
}

func (gradleRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	invoker, err := gradleInvoker(manifest)
	if err != nil {
		return err
	}
	args := append(invoker, "test")
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "gradle test passed")
	return nil
}

func (gradleRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	invoker, err := gradleInvoker(manifest)
	if err == nil {
		args := append(invoker, "clean")
		report.Commands = append(report.Commands, commandString(args))
		if output, cmdErr := runCommand(manifest.Dir, args); cmdErr != nil {
			return fmt.Errorf("%s\n%s", cmdErr, output)
		}
	}
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

func gradleInvoker(manifest *LoadedManifest) ([]string, error) {
	for _, wrapper := range []string{"gradlew", "gradlew.bat"} {
		path := filepath.Join(manifest.Dir, wrapper)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if runtime.GOOS == "windows" && strings.HasSuffix(wrapper, ".bat") {
				return []string{"cmd", "/c", wrapper}, nil
			}
			return []string{"./" + wrapper}, nil
		}
	}
	if _, err := exec.LookPath("gradle"); err == nil {
		return []string{"gradle"}, nil
	}
	return nil, fmt.Errorf("gradle runner requires gradlew or gradle on PATH")
}

func gradleArtifactCandidates(manifest *LoadedManifest) []string {
	name := manifest.BinaryName()
	return []string{
		filepath.Join(manifest.Dir, "build", "install", name, "bin", hostExecutableName(name)),
		filepath.Join(manifest.Dir, "build", "bin", hostExecutableName(name)),
		filepath.Join(manifest.Dir, "build", "compose", "binaries", "main", "app", name, hostExecutableName(name)),
	}
}

type dotnetRunner struct{}

func (dotnetRunner) check(manifest *LoadedManifest, ctx BuildContext) error {
	if err := requireRunnerCommands("dotnet"); err != nil {
		return err
	}
	csproj, err := dotnetProjectFile(manifest)
	if err != nil {
		return err
	}
	workload := requiredDotnetWorkload(csproj, ctx.Target)
	if workload == "" {
		return nil
	}
	output, err := runCommand(manifest.Dir, []string{"dotnet", "workload", "list"})
	if err != nil {
		return fmt.Errorf("dotnet workload list failed: %w", err)
	}
	if !strings.Contains(output, workload) {
		return fmt.Errorf("dotnet runner requires workload %q\n  install with: dotnet workload install %s", workload, workload)
	}
	return nil
}

func (dotnetRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	outputDir := filepath.Join(manifest.OpRoot(), "build", "dotnet")
	args := []string{"dotnet", "build", "-c", cmakeBuildConfig(ctx.Mode), "-o", outputDir}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if ctx.DryRun {
		return nil
	}
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	if err := syncDotnetArtifacts(manifest, outputDir); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "dotnet build complete")
	return nil
}

func (dotnetRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	args := []string{"dotnet", "test", "-c", cmakeBuildConfig(ctx.Mode)}
	report.Commands = append(report.Commands, commandString(args))
	ctx.Progress.Step(commandString(args))
	if output, err := runCommand(manifest.Dir, args); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "dotnet test passed")
	return nil
}

func (dotnetRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	args := []string{"dotnet", "clean"}
	report.Commands = append(report.Commands, commandString(args))
	if _, err := exec.LookPath("dotnet"); err == nil {
		if output, cmdErr := runCommand(manifest.Dir, args); cmdErr != nil {
			return fmt.Errorf("%s\n%s", cmdErr, output)
		}
	}
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

func dotnetProjectFile(manifest *LoadedManifest) (string, error) {
	matches, err := filepath.Glob(filepath.Join(manifest.Dir, "*.csproj"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("dotnet runner requires a .csproj file")
	}
	sort.Strings(matches)
	return matches[0], nil
}

func requiredDotnetWorkload(projectFile, target string) string {
	data, err := os.ReadFile(projectFile)
	if err != nil {
		return ""
	}
	content := string(data)
	if !strings.Contains(content, "UseMaui") && !strings.Contains(content, "Microsoft.Maui") {
		return ""
	}
	switch normalizePlatformName(target) {
	case "macos":
		return "maui-maccatalyst"
	case "ios":
		return "maui-ios"
	case "android":
		return "maui-android"
	default:
		return ""
	}
}

type qtCMakeRunner struct{}

func (qtCMakeRunner) check(_ *LoadedManifest, _ BuildContext) error {
	if err := requireRunnerCommands("cmake"); err != nil {
		return err
	}
	_, err := detectQt6Dir()
	return err
}

func (qtCMakeRunner) build(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	if err := ensureHostBuildTarget(RunnerQtCMake, ctx); err != nil {
		return err
	}
	qtDir, err := detectQt6Dir()
	if err != nil {
		return err
	}
	config := cmakeBuildConfig(ctx.Mode)
	binDir := filepath.Dir(manifest.BinaryPath())
	configureArgs := []string{
		"cmake",
		"-S", ".",
		"-B", manifest.CMakeBuildDir(),
		"-DCMAKE_BUILD_TYPE=" + config,
		"-DCMAKE_PREFIX_PATH=" + qtDir,
		"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY=" + binDir,
		"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY_DEBUG=" + binDir,
		"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY_RELEASE=" + binDir,
		"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY_RELWITHDEBINFO=" + binDir,
	}
	buildArgs := []string{"cmake", "--build", manifest.CMakeBuildDir(), "--config", config}
	report.Commands = append(report.Commands, commandString(configureArgs), commandString(buildArgs))
	if ctx.DryRun {
		return nil
	}
	if err := os.MkdirAll(manifest.CMakeBuildDir(), 0o755); err != nil {
		return err
	}
	ctx.Progress.Step(commandString(configureArgs))
	if output, err := runCommand(manifest.Dir, configureArgs); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	ctx.Progress.Step(commandString(buildArgs))
	if output, err := runCommand(manifest.Dir, buildArgs); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "qt-cmake build complete")
	return nil
}

func (qtCMakeRunner) test(manifest *LoadedManifest, ctx BuildContext, report *Report) error {
	runner := qtCMakeRunner{}
	if err := runner.build(manifest, ctx, report); err != nil {
		return err
	}
	config := cmakeBuildConfig(ctx.Mode)
	testArgs := []string{"ctest", "--test-dir", manifest.CMakeBuildDir(), "--output-on-failure", "-C", config}
	report.Commands = append(report.Commands, commandString(testArgs))
	ctx.Progress.Step(commandString(testArgs))
	if output, err := runCommand(manifest.Dir, testArgs); err != nil {
		return fmt.Errorf("%s\n%s", err, output)
	}
	report.Notes = append(report.Notes, "ctest passed")
	return nil
}

func (qtCMakeRunner) clean(manifest *LoadedManifest, _ BuildContext, report *Report) error {
	if err := os.RemoveAll(manifest.OpRoot()); err != nil {
		return err
	}
	report.Notes = append(report.Notes, "removed .op/")
	return nil
}

func detectQt6Dir() (string, error) {
	if qtDir := strings.TrimSpace(os.Getenv("Qt6_DIR")); qtDir != "" {
		if _, err := os.Stat(qtDir); err == nil {
			return qtDir, nil
		}
	}
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("brew"); err == nil {
			output, cmdErr := runCommand(".", []string{"brew", "--prefix", "qt6"})
			if cmdErr == nil {
				prefix := strings.TrimSpace(output)
				if prefix != "" {
					qtDir := filepath.Join(prefix, "lib", "cmake", "Qt6")
					if _, err := os.Stat(qtDir); err == nil {
						return qtDir, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("qt-cmake runner requires Qt6\n  install with: brew install qt6\n  then set: export Qt6_DIR=$(brew --prefix qt6)/lib/cmake/Qt6")
}

func copyWorkspaceIsolated(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".op", "node_modules", "venv", "__pycache__", "build", "dist":
				return filepath.SkipDir
			}
			return os.MkdirAll(targetPath, info.Mode())
		}
		if d.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}
		return copyFile(path, targetPath)
	})
}
