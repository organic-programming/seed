//go:build e2e

package codegen_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

type codegenHolonCase struct {
	slug       string
	genFile    string
	runtimeArg string
}

var codegenHolons = []codegenHolonCase{
	{slug: "gabriel-greeting-go", genFile: "examples/hello-world/gabriel-greeting-go/gen/go/greeting/v1/greeting_grpc.pb.go", runtimeArg: "version"},
	{slug: "clem-ader", genFile: "holons/clem-ader/gen/go/v1/holon_grpc.pb.go", runtimeArg: "version"},
	{slug: "matt-calculator-go", genFile: "examples/calculator/matt-calculator-go/gen/go/calculator/v1/calculator_grpc.pb.go", runtimeArg: "version"},
	{slug: "gabriel-greeting-cpp", genFile: "examples/hello-world/gabriel-greeting-cpp/gen/cpp/greeting/v1/greeting.grpc.pb.cc", runtimeArg: "version"},
	{slug: "gabriel-greeting-c", genFile: "examples/hello-world/gabriel-greeting-c/gen/c/greeting/v1/greeting.upb.c", runtimeArg: "version"},
	{slug: "gabriel-greeting-python", genFile: "examples/hello-world/gabriel-greeting-python/gen/python/greeting/v1/greeting_pb2_grpc.py", runtimeArg: "--help"},
	{slug: "gabriel-greeting-ruby", genFile: "examples/hello-world/gabriel-greeting-ruby/gen/ruby/greeting/v1/greeting_services_pb.rb", runtimeArg: "--help"},
	{slug: "gabriel-greeting-csharp", genFile: "examples/hello-world/gabriel-greeting-csharp/gen/csharp/GreetingGrpc.cs", runtimeArg: "--help"},
	{slug: "gabriel-greeting-dart", genFile: "examples/hello-world/gabriel-greeting-dart/gen/dart/greeting/v1/greeting.pbgrpc.dart", runtimeArg: "--help"},
	{slug: "gabriel-greeting-swift", genFile: "examples/hello-world/gabriel-greeting-swift/gen/swift/greeting/v1/greeting.grpc.swift", runtimeArg: "--help"},
	{slug: "gabriel-greeting-java", genFile: "examples/hello-world/gabriel-greeting-java/gen/java/greeting/v1/GreetingServiceGrpc.java", runtimeArg: "--help"},
	{slug: "gabriel-greeting-kotlin", genFile: "examples/hello-world/gabriel-greeting-kotlin/gen/kotlin/greeting/v1/GreetingGrpcKt.kt", runtimeArg: "--help"},
	{slug: "gabriel-greeting-node", genFile: "examples/hello-world/gabriel-greeting-node/gen/node/greeting/v1/greeting_grpc_pb.js", runtimeArg: "--help"},
	{slug: "gabriel-greeting-zig", genFile: "examples/hello-world/gabriel-greeting-zig/gen/c/greeting/v1/greeting.pb-c.c", runtimeArg: "version"},
	{slug: "gabriel-greeting-app-flutter", genFile: "examples/hello-world/gabriel-greeting-app-flutter/gen/dart/api/v1/holon.pbgrpc.dart"},
	{slug: "gabriel-greeting-app-swiftui", genFile: "examples/hello-world/gabriel-greeting-app-swiftui/gen/swift/api/v1/holon.grpc.swift"},
}

var codegenSDKs = []struct {
	lang    string
	version string
}{
	{lang: "go", version: "0.1.0"},
	{lang: "cpp", version: "1.80.0"},
	{lang: "c", version: "1.80.0"},
	{lang: "python", version: "0.1.0"},
	{lang: "ruby", version: "1.58.3"},
	{lang: "csharp", version: "0.1.0"},
	{lang: "dart", version: "0.1.0"},
	{lang: "swift", version: "0.1.0"},
	{lang: "java", version: "0.1.0"},
	{lang: "kotlin", version: "0.1.0"},
	{lang: "js", version: "0.1.0"},
	{lang: "zig", version: "0.1.0"},
}

func TestCodegen_CLI_RegeneratesCommittedStubs(t *testing.T) {
	sb := integration.NewSandbox(t)
	workspace := integration.DefaultWorkspaceDir(t)
	installCodegenSDKs(t, sb, workspace)

	for _, tc := range codegenHolons {
		t.Run(tc.slug, func(t *testing.T) {
			report := buildCodegenHolon(t, sb, tc.slug)
			if tc.runtimeArg != "" {
				runBuiltBinary(t, workspace, report, tc.runtimeArg)
			}

			genPath := filepath.Join(workspace, filepath.FromSlash(tc.genFile))
			before := fileSHA256(t, genPath)
			appendSentinel(t, genPath)

			buildCodegenHolon(t, sb, tc.slug)
			restored := fileSHA256(t, genPath)
			if restored != before {
				t.Fatalf("%s did not restore generated file %s: before=%s after=%s", tc.slug, tc.genFile, before, restored)
			}

			buildCodegenHolon(t, sb, tc.slug)
			noOp := fileSHA256(t, genPath)
			if noOp != before {
				t.Fatalf("%s second build changed generated file %s: before=%s after=%s", tc.slug, tc.genFile, before, noOp)
			}
		})
	}
}

func installCodegenSDKs(t *testing.T, sb *integration.Sandbox, workspace string) {
	t.Helper()
	target := hostTriplet(t)
	for _, sdk := range codegenSDKs {
		t.Run("sdk-"+sdk.lang, func(t *testing.T) {
			archive := filepath.Join(workspace, "dist", "sdk-prebuilts", sdk.lang, target, fmt.Sprintf("%s-holons-v%s-%s.tar.gz", sdk.lang, sdk.version, target))
			if _, err := os.Stat(archive); err == nil {
				result := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 20 * time.Minute}, "sdk", "install", sdk.lang, "--source", archive, "--version", sdk.version, "--target", target)
				integration.RequireSuccess(t, result)
				return
			} else if !os.IsNotExist(err) {
				t.Fatalf("stat SDK archive %s: %v", archive, err)
			}
			if installed := hostInstalledSDK(sdk.lang, sdk.version, target); installed != "" {
				dst := filepath.Join(sb.OPPATH, "sdk", sdk.lang, sdk.version, target)
				if err := copyTree(installed, dst); err != nil {
					t.Fatalf("copy installed SDK %s from %s: %v", sdk.lang, installed, err)
				}
				return
			}
			result := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 90 * time.Minute}, "sdk", "build", sdk.lang, "--version", sdk.version, "--target", target)
			integration.RequireSuccess(t, result)
		})
	}
}

func hostInstalledSDK(lang, version, target string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	root := filepath.Join(home, ".op", "sdk", lang, version, target)
	if _, err := os.Stat(filepath.Join(root, "manifest.json")); err == nil {
		return root
	}
	return ""
}

func copyTree(srcRoot, dstRoot string) error {
	return filepath.WalkDir(srcRoot, func(srcPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcRoot, srcPath)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstRoot, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(dstPath, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return err
			}
			return os.Symlink(target, dstPath)
		default:
			return copyFile(srcPath, dstPath, info.Mode())
		}
	})
}

func copyFile(srcPath, dstPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	return dst.Close()
}

func hostTriplet(t *testing.T) string {
	t.Helper()
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "aarch64-apple-darwin"
	case "darwin/amd64":
		return "x86_64-apple-darwin"
	case "linux/amd64":
		return "x86_64-unknown-linux-gnu"
	case "linux/arm64":
		return "aarch64-unknown-linux-gnu"
	case "windows/amd64":
		return "x86_64-pc-windows-msvc"
	default:
		t.Fatalf("unsupported host triplet for %s/%s", runtime.GOOS, runtime.GOARCH)
		return ""
	}
}

func buildCodegenHolon(t *testing.T, sb *integration.Sandbox, slug string) integration.LifecycleReport {
	t.Helper()
	result := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 45 * time.Minute}, "--format", "json", "build", slug)
	integration.RequireSuccess(t, result)
	return integration.DecodeJSON[integration.LifecycleReport](t, result.Stdout)
}

func runBuiltBinary(t *testing.T, workspace string, report integration.LifecycleReport, arg string) {
	t.Helper()
	if strings.TrimSpace(report.Binary) == "" {
		t.Fatalf("%s build did not report a binary", report.Holon)
	}
	binary := report.Binary
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(workspace, filepath.FromSlash(binary))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, arg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s binary failed to start with %q: %v\n%s", report.Holon, arg, err, string(out))
	}
	t.Logf("%s %s: %s", report.Holon, arg, strings.TrimSpace(string(out)))
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func appendSentinel(t *testing.T, path string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()
	if _, err := file.WriteString("\nOP_CODEGEN_NEGATIVE_TEST_SENTINEL\n"); err != nil {
		t.Fatalf("corrupt %s: %v", path, err)
	}
}
