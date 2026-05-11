//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
)

var sdks = []string{"c", "cpp", "csharp", "dart", "go", "java", "js", "js-web", "kotlin", "python", "ruby", "rust", "swift", "zig"}
var cppDownstream = []string{"cpp", "c", "ruby", "python", "csharp", "kotlin", "java", "js"}
var zigGRPCDownstream = append([]string{"zig"}, cppDownstream...)

var scriptToLang = map[string]string{
	".github/scripts/build-prebuilt-c.sh":      "c",
	".github/scripts/build-prebuilt-cpp.sh":    "cpp",
	".github/scripts/build-prebuilt-csharp.sh": "csharp",
	".github/scripts/build-prebuilt-dart.sh":   "dart",
	".github/scripts/build-prebuilt-go.sh":     "go",
	".github/scripts/build-prebuilt-java.sh":   "java",
	".github/scripts/build-prebuilt-js.sh":     "js",
	".github/scripts/build-prebuilt-js-web.sh": "js-web",
	".github/scripts/build-prebuilt-kotlin.sh": "kotlin",
	".github/scripts/build-prebuilt-python.sh": "python",
	".github/scripts/build-prebuilt-ruby.sh":   "ruby",
	".github/scripts/build-prebuilt-rust.sh":   "rust",
	".github/scripts/build-prebuilt-swift.sh":  "swift",
	".github/scripts/build-prebuilt-zig.sh":    "zig",
}

var republishAllPaths = map[string]bool{
	"seed-toolchain.yaml":                             true,
	".github/scripts/go.mod":                          true,
	".github/scripts/go.sum":                          true,
	".github/scripts/lib-codegen-prebuilt.sh":         true,
	".github/scripts/seed_release_bump.go":            true,
	".github/scripts/seed_toolchain.go":               true,
	".github/scripts/sdk_ci_paths.go":                 true,
	".github/scripts/build-prebuilt-codegen-light.sh": true,
	".github/scripts/promote-sdk-prebuilts.sh":        true,
	".github/workflows/_sdk-prebuilt-target.yml":      true,
	".github/workflows/sdk-prebuilts.yml":             true,
	".github/workflows/pipeline.yml":                  true,
}

var sdkDocOrAssetExtensions = map[string]bool{
	".md": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true, ".ico": true,
}

func normalizePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	for strings.HasPrefix(value, "./") {
		value = strings.TrimPrefix(value, "./")
	}
	return value
}

func isSDKDocOrAsset(value string) bool {
	value = normalizePath(value)
	name := strings.ToLower(path.Base(value))
	if sdkDocOrAssetExtensions[strings.ToLower(path.Ext(value))] {
		return true
	}
	if strings.HasPrefix(name, "readme") || strings.HasPrefix(name, "license") || strings.HasPrefix(name, "changelog") {
		return true
	}
	for _, part := range strings.Split(value, "/") {
		if strings.ToLower(part) == "docs" {
			return true
		}
	}
	return false
}

func isSDKSourcePath(value string) bool {
	value = normalizePath(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "sdk/") {
		return !isSDKDocOrAsset(value)
	}
	if value == "seed-toolchain.yaml" || value == ".gitmodules" {
		return true
	}
	if strings.HasPrefix(value, ".github/scripts/build-prebuilt-") && strings.HasSuffix(value, ".sh") {
		return true
	}
	if value == ".github/scripts/lib-codegen-prebuilt.sh" ||
		value == ".github/scripts/go.mod" ||
		value == ".github/scripts/go.sum" ||
		value == ".github/scripts/seed_release_bump.go" ||
		value == ".github/scripts/seed_toolchain.go" ||
		strings.HasPrefix(value, ".github/scripts/seed_toolchain/") ||
		value == ".github/scripts/sdk_ci_paths.go" {
		return true
	}
	if strings.HasPrefix(value, "holons/grace-op/internal/sdkprebuilts/") {
		return !strings.HasSuffix(strings.ToLower(value), ".md")
	}
	if strings.HasPrefix(value, "holons/grace-op/cmd/protoc-gen-op-adapter/") {
		return !strings.HasSuffix(strings.ToLower(value), ".md")
	}
	if strings.HasPrefix(value, "holons/grace-op/cmd/protoc-gen-op-noop/") {
		return !strings.HasSuffix(strings.ToLower(value), ".md")
	}
	return false
}

func sdkForPath(value string) string {
	value = normalizePath(value)
	if lang := scriptToLang[value]; lang != "" {
		return lang
	}
	for _, lang := range sdks {
		if strings.HasPrefix(value, "sdk/"+lang+"-holons/") {
			return lang
		}
	}
	return ""
}

func orderedUnion(groups ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, group := range groups {
		for _, item := range group {
			if !seen[item] {
				seen[item] = true
				out = append(out, item)
			}
		}
	}
	return out
}

func publishSet(paths []string) []string {
	normalized := []string{}
	for _, raw := range paths {
		value := normalizePath(raw)
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		return []string{}
	}
	for _, value := range normalized {
		if republishAllPaths[value] ||
			strings.HasPrefix(value, ".github/scripts/seed_toolchain/") ||
			strings.HasPrefix(value, "holons/grace-op/internal/sdkprebuilts/") ||
			strings.HasPrefix(value, "holons/grace-op/cmd/protoc-gen-op-adapter/") ||
			strings.HasPrefix(value, "holons/grace-op/cmd/protoc-gen-op-noop/") {
			return append([]string(nil), sdks...)
		}
	}
	groups := [][]string{}
	for _, value := range normalized {
		if strings.HasPrefix(value, "sdk/") && isSDKDocOrAsset(value) {
			continue
		}
		switch {
		case strings.HasPrefix(value, "sdk/cpp-holons/"):
			groups = append(groups, cppDownstream)
		case strings.HasPrefix(value, "sdk/zig-holons/third_party/grpc"):
			groups = append(groups, zigGRPCDownstream)
		default:
			if lang := sdkForPath(value); lang != "" {
				groups = append(groups, []string{lang})
			}
		}
	}
	return orderedUnion(groups...)
}

func readFilesArg(path string) ([]string, error) {
	var input *os.File
	if path == "" {
		input = os.Stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		input = file
	}
	out := []string{}
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		if value := strings.TrimSpace(scanner.Text()); value != "" {
			out = append(out, value)
		}
	}
	return out, scanner.Err()
}

func filesFlag(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--files" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("--files requires a path")
			}
			return args[i+1], nil
		}
		if strings.HasPrefix(args[i], "--files=") {
			return strings.TrimPrefix(args[i], "--files="), nil
		}
	}
	return "", nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sdk_ci_paths <classify|publish-set> [--files path]")
		os.Exit(2)
	}
	path, err := filesFlag(os.Args[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	files, err := readFilesArg(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "classify":
		sdkSource := false
		for _, file := range files {
			if isSDKSourcePath(file) {
				sdkSource = true
				break
			}
		}
		payload, _ := json.Marshal(sdkSource)
		fmt.Printf("sdk_source=%t\nsdk_source_json=%s\n", sdkSource, payload)
	case "publish-set":
		payload, _ := json.Marshal(publishSet(files))
		fmt.Println(string(payload))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
