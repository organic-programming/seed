//go:build e2e

package codegen_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
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

func TestCodegen_CLI_RegeneratesCommittedStubs(t *testing.T) {
	sb := integration.NewSandbox(t)
	workspace := integration.DefaultWorkspaceDir(t)

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
