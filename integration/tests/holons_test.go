package integration

import (
	"os/exec"
	"runtime"
	"testing"
)

type holonSpec struct {
	Slug      string
	Runner    string
	Requires  []string
	Platform  string
	Composite bool
}

type transportSpec struct {
	Name      string
	URIPrefix string
	Platform  string
}

var nativeHolons = []holonSpec{
	{Slug: "gabriel-greeting-go", Runner: "go-module", Requires: []string{"go"}},
	{Slug: "gabriel-greeting-c", Runner: "cmake", Requires: []string{"cmake"}},
	{Slug: "gabriel-greeting-cpp", Runner: "cmake", Requires: []string{"cmake"}},
	{Slug: "gabriel-greeting-rust", Runner: "cargo", Requires: []string{"cargo"}},
	{Slug: "gabriel-greeting-swift", Runner: "swift-package", Requires: []string{"swift"}},
	{Slug: "gabriel-greeting-python", Runner: "python", Requires: []string{"python3"}},
	{Slug: "gabriel-greeting-ruby", Runner: "ruby", Requires: []string{"ruby", "bundle"}},
	{Slug: "gabriel-greeting-node", Runner: "npm", Requires: []string{"node", "npm"}},
	{Slug: "gabriel-greeting-dart", Runner: "dart", Requires: []string{"dart"}},
	{Slug: "gabriel-greeting-java", Runner: "gradle", Requires: []string{"java", "gradle"}},
	{Slug: "gabriel-greeting-kotlin", Runner: "gradle", Requires: []string{"java", "gradle"}},
	{Slug: "gabriel-greeting-csharp", Runner: "dotnet", Requires: []string{"dotnet"}},
}

var compositeHolons = []holonSpec{
	{
		Slug:      "gabriel-greeting-app-swiftui",
		Runner:    "recipe",
		Platform:  "darwin",
		Composite: true,
		Requires: []string{
			"xcodebuild", "xcodegen", "go", "swift", "cargo", "python3",
			"cmake", "dotnet", "dart", "java", "gradle", "node", "npm",
			"ruby", "bundle",
		},
	},
}

var localTransports = []transportSpec{
	{Name: "auto", URIPrefix: "", Platform: ""},
	{Name: "grpc", URIPrefix: "grpc://", Platform: ""},
	{Name: "tcp", URIPrefix: "tcp://", Platform: ""},
	{Name: "stdio", URIPrefix: "stdio://", Platform: ""},
}

func nativeTestHolons(t *testing.T) []holonSpec {
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
			return []holonSpec{spec}
		}
	}
	return available[:1]
}

func lifecycleHolons(t *testing.T) []holonSpec {
	t.Helper()

	holons := append([]holonSpec{}, nativeTestHolons(t)...)
	if !testing.Short() {
		holons = append(holons, filterAvailableHolons(compositeHolons)...)
	}
	return holons
}

func compositeTestHolons(t *testing.T) []holonSpec {
	t.Helper()
	available := filterAvailableHolons(compositeHolons)
	if len(available) == 0 {
		t.Skip("no composite hello-world holons are buildable on this platform")
	}
	return available
}

func availableHelloWorldSlugs(t *testing.T, includeComposite bool) []string {
	t.Helper()

	sources := nativeHolons
	if includeComposite {
		sources = append(append([]holonSpec{}, nativeHolons...), compositeHolons...)
	}
	available := filterAvailableHolons(sources)
	slugs := make([]string, 0, len(available))
	for _, spec := range available {
		slugs = append(slugs, spec.Slug)
	}
	return slugs
}

func transportMatrix() []transportSpec {
	return filterAvailableTransports(localTransports)
}

func unixTransportAvailable() bool {
	return runtime.GOOS != "windows"
}

func filterAvailableHolons(specs []holonSpec) []holonSpec {
	out := make([]holonSpec, 0, len(specs))
	for _, spec := range specs {
		if !platformMatches(spec.Platform) {
			continue
		}
		if !toolsAvailable(spec.Requires...) {
			continue
		}
		out = append(out, spec)
	}
	return out
}

func filterAvailableTransports(specs []transportSpec) []transportSpec {
	out := make([]transportSpec, 0, len(specs))
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
