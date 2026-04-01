//go:build darwin

// Darwin-only COAX tests build the SwiftUI recipe app, launch the real app
// binary, and exercise its organism-level RPC and MCP surfaces.
package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const coaxAddress = "tcp://127.0.0.1:60000"

type coaxApp struct {
	address string
	process *processHandle
}

func TestCOAX_BuildRecipeApp(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	report := coaxBuildReportFor(t, sb, "gabriel-greeting-app-swiftui")
	requirePathExists(t, reportPath(report.Artifact))
}

func TestCOAX_RuntimeSurface(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	app := startCOAXApp(t, sb)
	defer app.process.Stop(t)

	t.Run("InspectServices", func(t *testing.T) {
		inspectResult := sb.runOP(t, "inspect", "127.0.0.1:60000")
		requireSuccess(t, inspectResult)
		requireContains(t, inspectResult.Stdout, "CoaxService")
		requireContains(t, inspectResult.Stdout, "GreetingAppService")
	})

	t.Run("ListMembers", func(t *testing.T) {
		listMembers := sb.runOP(t, app.address, "ListMembers", "{}")
		requireSuccess(t, listMembers)
	})

	t.Run("ConnectMember", func(t *testing.T) {
		connectMember := sb.runOP(t, app.address, "ConnectMember", `{"slug":"gabriel-greeting-go"}`)
		requireSuccess(t, connectMember)
	})

	t.Run("Greet", func(t *testing.T) {
		connectMember := sb.runOP(t, app.address, "ConnectMember", `{"slug":"gabriel-greeting-go"}`)
		requireSuccess(t, connectMember)

		greet := sb.runOP(t, app.address, "Greet", `{"name":"Bob"}`)
		requireSuccess(t, greet)
		requireContains(t, greet.Stdout, "Bob")
	})

	t.Run("MCPTools", func(t *testing.T) {
		responses, mcpResult := mcpConversation(t, sb, []string{app.address}, []map[string]any{
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
		requireSuccess(t, mcpResult)
		tools := responses[1]["result"].(map[string]any)["tools"].([]any)

		foundCoax := false
		foundGreet := false
		for _, entry := range tools {
			name := entry.(map[string]any)["name"].(string)
			if strings.Contains(name, "CoaxService.ListMembers") {
				foundCoax = true
			}
			if strings.Contains(name, "GreetingAppService.Greet") {
				foundGreet = true
			}
		}
		if !foundCoax || !foundGreet {
			t.Fatalf("COAX MCP tools missing expected reflected methods: %#v", tools)
		}
	})
}

func startCOAXApp(t *testing.T, sb *sandbox) *coaxApp {
	t.Helper()

	report := coaxBuildReportFor(t, sb, "gabriel-greeting-app-swiftui")
	appBundle := reportPath(report.Artifact)
	requirePathExists(t, appBundle)

	appBinary := appBundleExecutable(t, appBundle)
	home := filepath.Join(sb.Root, "home")
	tmpDir := filepath.Join(sb.Root, "tmp-home")
	for _, dir := range []string{home, tmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	process := sb.startProcess(t, runOptions{
		BinaryPath:       appBinary,
		SkipDiscoverRoot: true,
		Env: []string{
			"HOME=" + home,
			"CFFIXED_USER_HOME=" + home,
			"TMPDIR=" + tmpDir + string(os.PathSeparator),
		},
	})

	waitForCOAXReady(t, sb, process, coaxAddress, 2*time.Minute)
	return &coaxApp{
		address: coaxAddress,
		process: process,
	}
}

func waitForCOAXReady(t *testing.T, sb *sandbox, process *processHandle, address string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last cmdResult
	for time.Now().Before(deadline) {
		last = sb.runOPWithOptions(t, runOptions{Timeout: 2 * time.Second}, address, "ListMembers", "{}")
		if last.Err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf(
		"COAX app did not become ready within %s\napp stdout:\n%s\napp stderr:\n%s\nprobe stdout:\n%s\nprobe stderr:\n%s",
		timeout,
		process.Stdout(),
		process.Stderr(),
		last.Stdout,
		last.Stderr,
	)
}

func coaxBuildReportFor(t *testing.T, sb *sandbox, slug string) lifecycleReport {
	t.Helper()

	result := sb.runOPWithOptions(t, runOptions{Timeout: 15 * time.Minute}, "--format", "json", "build", slug)
	requireSuccess(t, result)
	return decodeJSON[lifecycleReport](t, result.Stdout)
}

func appBundleExecutable(t *testing.T, appBundle string) string {
	t.Helper()
	macosDir := filepath.Join(appBundle, "Contents", "MacOS")
	entries, err := os.ReadDir(macosDir)
	if err != nil {
		t.Fatalf("read %s: %v", macosDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		return filepath.Join(macosDir, entry.Name())
	}
	t.Fatalf("no executable found in %s", macosDir)
	return ""
}
