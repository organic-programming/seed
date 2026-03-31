//go:build darwin

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const coaxAddress = "tcp://127.0.0.1:60000"

func TestCOAX_BuildAndRPC(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	report := buildReportFor(t, sb, "gabriel-greeting-app-swiftui")
	appBundle := reportPath(report.Artifact)
	requirePathExists(t, appBundle)

	appBinary := appBundleExecutable(t, appBundle)
	home := t.TempDir()
	tmpDir := filepath.Join(home, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir tmp dir: %v", err)
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
	defer process.Stop(t)

	waitUntil(t, 2*time.Minute, func() bool {
		result := sb.runOPWithOptions(t, runOptions{Timeout: 2 * time.Second}, coaxAddress, "ListMembers")
		return result.Err == nil
	})

	inspectResult := sb.runOP(t, "inspect", "127.0.0.1:60000")
	requireSuccess(t, inspectResult)
	requireContains(t, inspectResult.Stdout, "CoaxService")
	requireContains(t, inspectResult.Stdout, "GreetingAppService")

	listMembers := sb.runOP(t, coaxAddress, "ListMembers", "{}")
	requireSuccess(t, listMembers)

	connectMember := sb.runOP(t, coaxAddress, "ConnectMember", `{"slug":"gabriel-greeting-go"}`)
	requireSuccess(t, connectMember)

	greet := sb.runOP(t, coaxAddress, "Greet", `{"name":"Bob"}`)
	requireSuccess(t, greet)
	requireContains(t, greet.Stdout, "Bob")

	responses, mcpResult := mcpConversation(t, sb, []string{coaxAddress}, []map[string]any{
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
