package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type LifecycleReport struct {
	Operation   string            `json:"operation"`
	Target      string            `json:"target"`
	Holon       string            `json:"holon"`
	Dir         string            `json:"dir"`
	Manifest    string            `json:"manifest"`
	Runner      string            `json:"runner,omitempty"`
	Kind        string            `json:"kind,omitempty"`
	Binary      string            `json:"binary,omitempty"`
	BuildTarget string            `json:"build_target,omitempty"`
	BuildMode   string            `json:"build_mode,omitempty"`
	Artifact    string            `json:"artifact,omitempty"`
	Commands    []string          `json:"commands,omitempty"`
	Notes       []string          `json:"notes,omitempty"`
	Children    []LifecycleReport `json:"children,omitempty"`
}

type InstallReport struct {
	Operation   string   `json:"operation"`
	Target      string   `json:"target"`
	Holon       string   `json:"holon"`
	Dir         string   `json:"dir,omitempty"`
	Manifest    string   `json:"manifest,omitempty"`
	Binary      string   `json:"binary,omitempty"`
	BuildTarget string   `json:"build_target,omitempty"`
	BuildMode   string   `json:"build_mode,omitempty"`
	Artifact    string   `json:"artifact,omitempty"`
	Installed   string   `json:"installed,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

type DiscoverJSON struct {
	Entries []struct {
		Slug         string `json:"slug"`
		RelativePath string `json:"relative_path"`
		Origin       string `json:"origin"`
	} `json:"entries"`
	InstalledBinaries []string `json:"installed_binaries,omitempty"`
	PathBinaries      []string `json:"path_binaries,omitempty"`
}

type ListJSON struct {
	Entries []struct {
		Identity struct {
			UUID       string `json:"uuid"`
			GivenName  string `json:"givenName"`
			FamilyName string `json:"familyName"`
		} `json:"identity"`
		RelativePath string `json:"relativePath"`
		Origin       string `json:"origin"`
	} `json:"entries"`
}

type HolonSpec struct {
	Slug      string
	Runner    string
	Requires  []string
	Platform  string
	Composite bool
	// HolonPath is the workspace-relative path to the holon directory.
	// When empty, defaults to "examples/hello-world/<Slug>".
	HolonPath string
}

type TransportSpec struct {
	Name      string
	URIPrefix string
	Platform  string
}

var nativeHolons = []HolonSpec{
	{Slug: "gabriel-greeting-go", Runner: "go-module", Requires: []string{"go"}},
	{Slug: "gabriel-greeting-c", Runner: "cmake", Requires: []string{"cmake"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-cpp", Runner: "cmake", Requires: []string{"cmake"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-rust", Runner: "cargo", Requires: []string{"cargo"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-zig", Runner: "zig", Requires: []string{"zig", "cmake", "ninja"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-swift", Runner: "swift-package", Requires: []string{"swift"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-python", Runner: "python", Requires: []string{"python3"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-ruby", Runner: "ruby", Requires: []string{"ruby", "bundle"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-node", Runner: "npm", Requires: []string{"node", "npm"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-dart", Runner: "dart", Requires: []string{"dart"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-java", Runner: "gradle", Requires: []string{"java", "gradle"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-kotlin", Runner: "gradle", Requires: []string{"java", "gradle"}, Platform: "darwin"},
	{Slug: "gabriel-greeting-csharp", Runner: "dotnet", Requires: []string{"dotnet"}, Platform: "darwin"},
	{Slug: "matt-calculator-go", Runner: "go-module", Requires: []string{"go"}, HolonPath: "examples/calculator/matt-calculator-go"},
}

var compositeHolons = []HolonSpec{
	{
		Slug:      "gabriel-greeting-app-swiftui",
		Runner:    "recipe",
		Platform:  "darwin",
		Composite: true,
		Requires: []string{
			"xcodebuild", "xcodegen", "go", "swift", "cargo", "zig", "python3",
			"cmake", "ninja", "dotnet", "dart", "java", "gradle", "node", "npm",
			"ruby", "bundle",
		},
	},
	{
		Slug:      "gabriel-greeting-app-flutter",
		Runner:    "recipe",
		Platform:  "darwin",
		Composite: true,
		Requires: []string{
			"flutter", "go", "swift", "cargo", "zig", "python3",
			"cmake", "ninja", "dotnet", "dart", "java", "gradle", "node", "npm",
			"ruby", "bundle",
		},
	},
	{
		Slug:      "gabriel-greeting-app-kotlin-compose",
		Runner:    "recipe",
		Platform:  "darwin",
		Composite: true,
		Requires: []string{
			"go", "swift", "cargo", "python3",
			"cmake", "dotnet", "dart", "java", "gradle", "node", "npm",
			"ruby", "bundle",
		},
	},
}

var localTransports = []TransportSpec{
	{Name: "auto", URIPrefix: "", Platform: ""},
	{Name: "tcp", URIPrefix: "tcp://", Platform: ""},
	{Name: "stdio", URIPrefix: "stdio://", Platform: ""},
}

func NativeTestHolons(t *testing.T) []HolonSpec {
	t.Helper()
	available := filterAvailableHolons(nativeHolons)
	if len(available) == 0 {
		t.Skip("no hello-world native holons are buildable on this machine")
	}
	if !testing.Short() {
		return available
	}
	for _, spec := range available {
		if spec.Slug == "gabriel-greeting-go" {
			return []HolonSpec{spec}
		}
	}
	return available[:1]
}

func LifecycleHolons(t *testing.T) []HolonSpec {
	t.Helper()
	holons := append([]HolonSpec{}, NativeTestHolons(t)...)
	if !testing.Short() {
		holons = append(holons, filterAvailableHolons(compositeHolons)...)
	}
	return holons
}

func CompositeTestHolons(t *testing.T) []HolonSpec {
	t.Helper()
	available := filterAvailableHolons(compositeHolons)
	if len(available) == 0 {
		t.Skip("no composite hello-world holons are buildable on this platform")
	}
	return available
}

func SupportsOPTest(spec HolonSpec) bool {
	switch spec.Runner {
	case "python", "dart":
		return false
	default:
		return true
	}
}

func AvailableHelloWorldSlugs(t *testing.T, includeComposite bool) []string {
	t.Helper()
	sources := nativeHolons
	if includeComposite {
		sources = append(append([]HolonSpec{}, nativeHolons...), compositeHolons...)
	}
	available := filterAvailableHolons(sources)
	slugs := make([]string, 0, len(available))
	for _, spec := range available {
		slugs = append(slugs, spec.Slug)
	}
	return slugs
}

func HolonPathForSlug(slug string) string {
	sources := append(append([]HolonSpec{}, nativeHolons...), compositeHolons...)
	for _, spec := range sources {
		if spec.Slug == slug {
			if spec.HolonPath != "" {
				return filepath.FromSlash(spec.HolonPath)
			}
			break
		}
	}
	return filepath.Join("examples", "hello-world", filepath.FromSlash(slug))
}

func TransportMatrix() []TransportSpec {
	return filterAvailableTransports(localTransports)
}

func UnixTransportAvailable() bool {
	return runtime.GOOS != "windows"
}

func CompositeArtifactPath(rootPath string, slug string) string {
	switch slug {
	case "gabriel-greeting-app-swiftui":
		return filepath.Join(rootPath, "examples", "hello-world", slug, ".op", "build", "GabrielGreetingApp.app")
	default:
		return filepath.Join(rootPath, "examples", "hello-world", slug, ".op", "build", slug+".holon")
	}
}

func BuildReportFor(t *testing.T, sb *Sandbox, slug string, extraArgs ...string) LifecycleReport {
	t.Helper()
	args := append([]string{"--format", "json", "build"}, extraArgs...)
	args = append(args, slug)
	result := sb.RunOP(t, args...)
	RequireSuccess(t, result)
	return DecodeJSON[LifecycleReport](t, result.Stdout)
}

func BuildDryRunReportFor(t *testing.T, sb *Sandbox, slug string, extraArgs ...string) LifecycleReport {
	t.Helper()
	args := append([]string{"--format", "json", "build", "--dry-run"}, extraArgs...)
	args = append(args, slug)
	result := sb.RunOP(t, args...)
	RequireSuccess(t, result)
	return DecodeJSON[LifecycleReport](t, result.Stdout)
}

func InstallReportFor(t *testing.T, sb *Sandbox, args ...string) InstallReport {
	t.Helper()
	fullArgs := append([]string{"--format", "json", "install"}, args...)
	result := sb.RunOP(t, fullArgs...)
	RequireSuccess(t, result)
	return DecodeJSON[InstallReport](t, result.Stdout)
}

func CleanHolon(t *testing.T, sb *Sandbox, slug string) {
	t.Helper()
	result := sb.RunOP(t, "clean", slug)
	RequireSuccess(t, result)
}

func BinaryPathFor(t *testing.T, sb *Sandbox, slug string) string {
	t.Helper()
	report := BuildDryRunReportFor(t, sb, slug)
	return ReportPath(t, report.Binary)
}

func ArtifactPathFor(t *testing.T, sb *Sandbox, slug string) string {
	t.Helper()
	report := BuildDryRunReportFor(t, sb, slug)
	return ReportPath(t, report.Artifact)
}

func RemoveArtifactFor(t *testing.T, sb *Sandbox, slug string) {
	t.Helper()
	path := ArtifactPathFor(t, sb, slug)
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("remove artifact %s: %v", path, err)
	}
}

func InstalledNameFor(t *testing.T, sb *Sandbox, slug string) string {
	t.Helper()
	report := InstallReportFor(t, sb, "--build", slug)
	return report.Installed
}

func ReadDiscoverJSON(t *testing.T, sb *Sandbox) DiscoverJSON {
	t.Helper()
	result := sb.RunOP(t, "--format", "json", "discover")
	RequireSuccess(t, result)
	return DecodeJSON[DiscoverJSON](t, result.Stdout)
}

func ReadListJSON(t *testing.T, sb *Sandbox) ListJSON {
	t.Helper()
	result := sb.RunOP(t, "--format", "json", "list")
	RequireSuccess(t, result)
	return DecodeJSON[ListJSON](t, result.Stdout)
}

func ReportPath(t *testing.T, path string) string {
	t.Helper()
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(DefaultWorkspaceDir(t), filepath.FromSlash(path))
}

func PathWithPrepend(dir string) string {
	if strings.TrimSpace(dir) == "" {
		return os.Getenv("PATH")
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

func InstallFakeGitLSRemote(t *testing.T, entries map[string][]string) string {
	t.Helper()

	binDir := t.TempDir()
	realGit, _ := exec.LookPath("git")
	if runtime.GOOS == "windows" {
		scriptPath := filepath.Join(binDir, "git.bat")
		var body strings.Builder
		body.WriteString("@echo off\r\n")
		body.WriteString("if not \"%1\"==\"ls-remote\" goto unsupported\r\n")
		body.WriteString("if not \"%2\"==\"--tags\" goto unsupported\r\n")
		body.WriteString("if not \"%3\"==\"--refs\" goto unsupported\r\n")
		body.WriteString("set url=%4\r\n")
		for url, tags := range entries {
			body.WriteString(fmt.Sprintf("if \"%%url%%\"==\"%s\" (\r\n", url))
			for _, tag := range tags {
				body.WriteString(fmt.Sprintf("  echo deadbeef refs/tags/%s\r\n", tag))
			}
			body.WriteString("  exit /b 0\r\n")
			body.WriteString(")\r\n")
		}
		body.WriteString(":unsupported\r\n")
		if strings.TrimSpace(realGit) != "" {
			body.WriteString(fmt.Sprintf("\"%s\" %%*\r\n", realGit))
			body.WriteString("exit /b %errorlevel%\r\n")
		}
		body.WriteString("echo unsupported git args %* 1>&2\r\n")
		body.WriteString("exit /b 1\r\n")
		if err := os.WriteFile(scriptPath, []byte(body.String()), 0o755); err != nil {
			t.Fatalf("write fake git.bat: %v", err)
		}
		return binDir
	}

	scriptPath := filepath.Join(binDir, "git")
	var body strings.Builder
	body.WriteString("#!/bin/sh\n")
	body.WriteString("if [ \"$1\" = \"ls-remote\" ] && [ \"$2\" = \"--tags\" ] && [ \"$3\" = \"--refs\" ]; then\n")
	body.WriteString("  case \"$4\" in\n")
	for url, tags := range entries {
		body.WriteString(fmt.Sprintf("    %q)\n", url))
		for _, tag := range tags {
			body.WriteString(fmt.Sprintf("      printf 'deadbeef refs/tags/%s\\n'\n", tag))
		}
		body.WriteString("      exit 0\n")
		body.WriteString("      ;;\n")
	}
	body.WriteString("  esac\n")
	body.WriteString("fi\n")
	if strings.TrimSpace(realGit) != "" {
		body.WriteString(fmt.Sprintf("exec %q \"$@\"\n", realGit))
	}
	body.WriteString("echo \"unsupported git args: $*\" >&2\n")
	body.WriteString("exit 1\n")
	if err := os.WriteFile(scriptPath, []byte(body.String()), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	return binDir
}

func filterAvailableHolons(specs []HolonSpec) []HolonSpec {
	out := make([]HolonSpec, 0, len(specs))
	for _, spec := range specs {
		if !platformMatches(spec.Platform) {
			continue
		}
		if !toolsAvailable(spec.Requires...) {
			continue
		}
		if !exampleHolonExists(spec) {
			continue
		}
		out = append(out, spec)
	}
	return out
}

// exampleHolonExists returns true when the holon directory can be found on
// disk. For holons with an explicit HolonPath the path is used as-is; for
// the legacy hello-world holons the slug is looked up under examples/hello-world/.
func exampleHolonExists(spec HolonSpec) bool {
	if spec.HolonPath != "" {
		return holonDirExists(spec.HolonPath)
	}
	return holonDirExists(filepath.Join("examples", "hello-world", spec.Slug))
}

// holonDirExists returns true when relPath (relative to the workspace root)
// resolves to an existing directory.
func holonDirExists(relPath string) bool {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return true
	}
	// fixtures.go lives at <root>/ader/catalogues/grace-op/integration/fixtures.go
	// → go up 4 levels to reach the workspace root.
	holonDir := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", filepath.FromSlash(relPath))
	info, err := os.Stat(holonDir)
	return err == nil && info.IsDir()
}

func filterAvailableTransports(specs []TransportSpec) []TransportSpec {
	out := make([]TransportSpec, 0, len(specs))
	for _, spec := range specs {
		if !platformMatches(spec.Platform) {
			continue
		}
		out = append(out, spec)
	}
	return out
}

func platformMatches(platform string) bool {
	switch platform {
	case "", "all":
		return true
	case "!windows":
		return runtime.GOOS != "windows"
	default:
		return runtime.GOOS == platform
	}
}

func toolsAvailable(names ...string) bool {
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			return false
		}
	}
	return true
}

func SleepForFileSystem() {
	time.Sleep(50 * time.Millisecond)
}
