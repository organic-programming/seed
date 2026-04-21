package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/identity"
)

func TestMCPCommandToolsList(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)

	responses, stderr, exitCode := runMCPConversation(t, []string{
		"mcp", "echo-server",
	}, []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]any{},
		},
	})

	if exitCode != 0 {
		t.Fatalf("mcp exit code = %d, stderr=%q", exitCode, stderr)
	}
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}

	toolsResult, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list result = %#v, want map", responses[1]["result"])
	}
	toolsPayload, ok := toolsResult["tools"].([]any)
	if !ok || len(toolsPayload) != 1 {
		t.Fatalf("tools/list tools = %#v, want one tool", toolsResult["tools"])
	}
	tool, ok := toolsPayload[0].(map[string]any)
	if !ok {
		t.Fatalf("tool payload = %#v, want map", toolsPayload[0])
	}
	if tool["name"] != "echo-server.EchoService.Ping" {
		t.Fatalf("tool name = %#v, want echo-server.EchoService.Ping", tool["name"])
	}
	if !strings.Contains(fmt.Sprint(tool["description"]), "Echo the request payload.") {
		t.Fatalf("tool description = %#v", tool["description"])
	}
	inputSchema, ok := tool["inputSchema"].(map[string]any)
	if !ok {
		t.Fatalf("inputSchema = %#v, want map", tool["inputSchema"])
	}
	required, ok := inputSchema["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "message" {
		t.Fatalf("required = %#v, want [message]", inputSchema["required"])
	}
}

func TestMCPCommandToolsCall(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)

	responses, stderr, exitCode := runMCPConversation(t, []string{
		"mcp", "echo-server",
	}, []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "echo-server.EchoService.Ping",
				"arguments": map[string]any{
					"message": "Hello",
					"tags":    []string{"a", "b"},
					"mode":    "ECHO_MODE_UPPER",
				},
			},
		},
	})

	if exitCode != 0 {
		t.Fatalf("mcp exit code = %d, stderr=%q", exitCode, stderr)
	}
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}

	callResult, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call result = %#v, want map", responses[1]["result"])
	}
	structured, ok := callResult["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent = %#v, want map", callResult["structuredContent"])
	}
	if structured["message"] != "HELLO" {
		t.Fatalf("message = %#v, want HELLO", structured["message"])
	}
	if structured["count"] != float64(2) {
		t.Fatalf("count = %#v, want 2", structured["count"])
	}
	if structured["mode"] != "ECHO_MODE_UPPER" {
		t.Fatalf("mode = %#v, want ECHO_MODE_UPPER", structured["mode"])
	}
}

func TestMCPCommandMultiHolonToolsList(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)
	seedRobGoHolon(t, root)

	responses, stderr, exitCode := runMCPConversation(t, []string{
		"mcp", "echo-server", "rob-go",
	}, []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]any{},
		},
	})

	if exitCode != 0 {
		t.Fatalf("mcp exit code = %d, stderr=%q", exitCode, stderr)
	}
	toolsResult := responses[1]["result"].(map[string]any)
	toolsPayload := toolsResult["tools"].([]any)

	var names []string
	for _, entry := range toolsPayload {
		names = append(names, entry.(map[string]any)["name"].(string))
	}
	if !containsString(names, "echo-server.EchoService.Ping") {
		t.Fatalf("tool names missing echo-server: %#v", names)
	}
	if !containsString(names, "rob-go.RobGoService.Build") {
		t.Fatalf("tool names missing rob-go: %#v", names)
	}
}

func TestMCPCommandPromptsExposeSkills(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)

	responses, stderr, exitCode := runMCPConversation(t, []string{
		"mcp", "echo-server",
	}, []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "prompts/list",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "prompts/get",
			"params": map[string]any{
				"name": "echo-server.repeat-back",
			},
		},
	})

	if exitCode != 0 {
		t.Fatalf("mcp exit code = %d, stderr=%q", exitCode, stderr)
	}

	promptsResult := responses[1]["result"].(map[string]any)
	promptsPayload := promptsResult["prompts"].([]any)
	if len(promptsPayload) != 1 {
		t.Fatalf("prompts/list = %#v, want one prompt", promptsPayload)
	}
	if promptsPayload[0].(map[string]any)["name"] != "echo-server.repeat-back" {
		t.Fatalf("prompt name = %#v, want echo-server.repeat-back", promptsPayload[0])
	}

	promptResult := responses[2]["result"].(map[string]any)
	messages := promptResult["messages"].([]any)
	content := messages[0].(map[string]any)["content"].(map[string]any)
	text := content["text"].(string)
	if !strings.Contains(text, "Call EchoService.Ping with the text to echo.") {
		t.Fatalf("prompt text missing steps: %q", text)
	}
	if !strings.Contains(text, "echo-server.EchoService.Ping") {
		t.Fatalf("prompt text missing tool references: %q", text)
	}
}

func TestMCPCommandListsProtoBackedSequenceTool(t *testing.T) {
	repoRoot := inspectRepoRoot(t)
	chdirForTest(t, repoRoot)

	responses, stderr, exitCode := runMCPConversation(t, []string{
		"mcp", "gabriel-greeting-go",
	}, []map[string]any{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]any{},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]any{},
		},
	})

	if exitCode != 0 {
		t.Fatalf("mcp exit code = %d, stderr=%q", exitCode, stderr)
	}

	toolsResult := responses[1]["result"].(map[string]any)
	toolsPayload := toolsResult["tools"].([]any)

	var names []string
	for _, entry := range toolsPayload {
		names = append(names, entry.(map[string]any)["name"].(string))
	}
	if !containsString(names, "gabriel-greeting-go.sequence.multilingual-greeting") {
		t.Fatalf("tool names missing sequence tool: %#v", names)
	}
}

func TestToolsCommandOpenAIFormat(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)

	output := captureStdout(t, func() {
		code := Run([]string{"tools", "echo-server", "--format", "openai"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("tools returned %d, want 0", code)
		}
	})

	var payload []struct {
		Type     string `json:"type"`
		Function struct {
			Name       string         `json:"name"`
			Parameters map[string]any `json:"parameters"`
		} `json:"function"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("tools output is invalid json: %v\noutput=%s", err, output)
	}
	if len(payload) != 1 || payload[0].Type != "function" {
		t.Fatalf("unexpected tools payload: %+v", payload)
	}
	if payload[0].Function.Name != "echo-server.EchoService.Ping" {
		t.Fatalf("function name = %q, want echo-server.EchoService.Ping", payload[0].Function.Name)
	}
}

func runMCPConversation(t *testing.T, args []string, requests []map[string]any) ([]map[string]any, string, int) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	runtimeHome := filepath.Join(cwd, ".runtime")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OPBIN", filepath.Join(runtimeHome, "bin"))

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	oldStdin, oldStdout, oldStderr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = stdinReader, stdoutWriter, stderrWriter
	t.Cleanup(func() {
		os.Stdin, os.Stdout, os.Stderr = oldStdin, oldStdout, oldStderr
	})

	done := make(chan int, 1)
	go func() {
		done <- Run(args, "0.1.0-test")
		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
	}()

	encoder := json.NewEncoder(stdinWriter)
	for _, request := range requests {
		if err := encoder.Encode(request); err != nil {
			t.Fatal(err)
		}
	}
	_ = stdinWriter.Close()

	var responses []map[string]any
	scanner := bufio.NewScanner(stdoutReader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("invalid MCP response %q: %v", line, err)
		}
		responses = append(responses, payload)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	stderrBytes, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}

	return responses, string(stderrBytes), <-done
}

func seedEchoHolon(t *testing.T, root string) {
	t.Helper()

	targetDir := filepath.Join(root, "holons", "echo-server")
	sourceDir := cliTestSupportDir(t, "echoholon")
	copyDir(t, sourceDir, targetDir)
	legacyManifestPath := filepath.Join(targetDir, "holon."+"yaml")
	manifestData, err := os.ReadFile(legacyManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(targetDir, identity.ManifestFileName), string(manifestData)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(legacyManifestPath); err != nil {
		t.Fatal(err)
	}

	binaryPath := filepath.Join(targetDir, ".op", "build", "bin", "echo-server")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}

	repoRoot := graceOpRepoRoot(t)
	cmd := exec.Command("go", "build", "-o", binaryPath, "./internal/cli/testsupport/echoholon")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build echo holon: %v\n%s", err, string(output))
	}
}

func seedRobGoHolon(t *testing.T, root string) {
	t.Helper()

	dir := filepath.Join(root, "holons", "rob-go")
	if err := os.MkdirAll(filepath.Join(dir, "protos", "rob_go", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `schema: holon/v0
uuid: "mcp-test-rob-go"
given_name: "rob"
family_name: "go"
motto: "Build what you mean."
composer: "test"
clade: "deterministic/io_bound"
status: draft
born: "2026-03-08"
parents: []
reproduction: manual
aliases: ["rob-go", "rob"]
generated_by: "test"
lang: "go"
proto_status: draft
kind: native
build:
  runner: go-module
artifacts:
  binary: rob-go
`
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}

	proto := `syntax = "proto3";

package rob_go.v1;

// Wraps the go command for gRPC access.
service RobGoService {
  // Compile Go packages.
  rpc Build(BuildRequest) returns (BuildResponse);
}

message BuildRequest {
  // The Go package to build.
  // @required
  string package = 1;
}

message BuildResponse {
  // Compiler output.
  string output = 1;
}
`
	if err := os.WriteFile(filepath.Join(dir, "protos", "rob_go", "v1", "rob_go.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	binaryPath := filepath.Join(dir, ".op", "build", "bin", "rob-go")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}

	repoRoot := graceOpRepoRoot(t)
	cmd := exec.Command("go", "build", "-o", binaryPath, "./internal/cli/testsupport/describeonly")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build rob-go holon: %v\n%s", err, string(output))
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()

	if err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	}); err != nil {
		t.Fatal(err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func graceOpRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func cliTestSupportDir(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testsupport", name)
}
