//go:build e2e

package invoke_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInvoke_CLI_OPServiceAcrossTransports(t *testing.T) {
	fixtureCases := []struct {
		name    string
		workDir func(t *testing.T, f invokeOPFixture) string
		prepare func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture)
		payload func(t *testing.T, f invokeOPFixture) string
		assert  func(t *testing.T, result integration.CmdResult, f invokeOPFixture)
	}{
		{
			name:    "Discover",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"root_dir": f.Workspace})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				entries, ok := payload["entries"].([]any)
				if !ok || len(entries) == 0 {
					t.Fatalf("entries = %#v, want non-empty", payload["entries"])
				}
			},
		},
		{
			name:    "Invoke",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"holon": "go", "args": []string{"version"}})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if !strings.Contains(payload["stdout"].(string), "go version") {
					t.Fatalf("stdout = %#v, want go version", payload["stdout"])
				}
			},
		},
		{
			name:    "CreateIdentity",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ScratchDir },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"given_name": "Alpha", "family_name": "Builder", "motto": "Builds holons.", "composer": "test", "clade": "DETERMINISTIC_IO_BOUND", "lang": "go", "output_dir": filepath.Join(f.ScratchDir, "created")})
			},
			assert: func(t *testing.T, result integration.CmdResult, f invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if payload["filePath"] == "" {
					t.Fatalf("unexpected create identity payload: %#v", payload)
				}
				if _, err := os.Stat(filepath.Join(f.ScratchDir, "created", "holon.proto")); err != nil {
					t.Fatalf("created holon manifest missing: %v", err)
				}
			},
		},
		{
			name:    "ListIdentities",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"root_dir": f.Workspace})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				entries, ok := payload["entries"].([]any)
				if !ok || len(entries) == 0 {
					t.Fatalf("entries = %#v, want non-empty", payload["entries"])
				}
			},
		},
		{
			name:    "ShowIdentity",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string { return mustJSON(t, map[string]any{"uuid": f.ShowUUID}) },
			assert: func(t *testing.T, result integration.CmdResult, f invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if payload["filePath"] == "" || payload["rawContent"] == "" {
					t.Fatalf("unexpected show payload: %#v", payload)
				}
				identity, ok := payload["identity"].(map[string]any)
				if !ok || identity["uuid"] != f.ShowUUID {
					t.Fatalf("identity = %#v, want uuid %s", payload["identity"], f.ShowUUID)
				}
			},
		},
		{
			name:    "ListTemplates",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string { return "{}" },
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				entries, ok := payload["entries"].([]any)
				if !ok || len(entries) == 0 {
					t.Fatalf("entries = %#v, want non-empty", payload["entries"])
				}
			},
		},
		{
			name:    "GenerateTemplate",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ScratchDir },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"template": "coax-swiftui", "slug": "my-console", "dir": filepath.Join(f.ScratchDir, "templates")})
			},
			assert: func(t *testing.T, result integration.CmdResult, f invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if payload["dir"] == "" {
					t.Fatalf("unexpected template payload: %#v", payload)
				}
				if _, err := os.Stat(filepath.Join(f.ScratchDir, "templates", "my-console")); err != nil {
					t.Fatalf("generated template missing: %v", err)
				}
			},
		},
		{
			name:    "Version",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string { return "{}" },
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if payload["name"] != "op" || payload["banner"] == "" {
					t.Fatalf("unexpected version payload: %#v", payload)
				}
			},
		},
		{
			name:    "Check",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"target": "gabriel-greeting-go"})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				report := payload["report"].(map[string]any)
				if report["operation"] != "check" {
					t.Fatalf("operation = %#v, want check", report["operation"])
				}
			},
		},
		{
			name:    "Build",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"target": "gabriel-greeting-go", "build": map[string]any{"dry_run": true}})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				report := payload["report"].(map[string]any)
				if report["operation"] != "build" {
					t.Fatalf("operation = %#v, want build", report["operation"])
				}
			},
		},
		{
			name:    "Test",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"target": f.TestTarget})
			},
			assert: func(t *testing.T, result integration.CmdResult, f invokeOPFixture) {
				t.Helper()
				if f.TestTarget == "" {
					t.Skip("no suitable non-go holon available for op test")
				}
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				report := payload["report"].(map[string]any)
				if report["operation"] != "test" {
					t.Fatalf("operation = %#v, want test", report["operation"])
				}
			},
		},
		{
			name:    "Clean",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				integration.BuildReportFor(t, sb, "gabriel-greeting-go")
			},
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"target": "gabriel-greeting-go"})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				report := payload["report"].(map[string]any)
				if report["operation"] != "clean" {
					t.Fatalf("operation = %#v, want clean", report["operation"])
				}
			},
		},
		{
			name:    "Install",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"target": "gabriel-greeting-go", "build": true})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				report := payload["report"].(map[string]any)
				if report["installed"] == "" {
					t.Fatalf("installed path missing in payload: %#v", payload)
				}
			},
		},
		{
			name:    "Uninstall",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				integration.InstallReportFor(t, sb, "--build", "gabriel-greeting-go")
			},
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"target": "gabriel-greeting-go.holon"})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				report := payload["report"].(map[string]any)
				if report["installed"] == "" {
					t.Fatalf("installed path missing in payload: %#v", payload)
				}
			},
		},
		{
			name:    "Run",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				integration.RemoveArtifactFor(t, sb, "gabriel-greeting-go")
			},
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"holon": "gabriel-greeting-go", "no_build": true})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireFailure(t, result)
				integration.RequireContains(t, result.Stderr, "artifact missing")
			},
		},
		{
			name:    "Inspect",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"target": "gabriel-greeting-go"})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				document := payload["document"].(map[string]any)
				services, ok := document["services"].([]any)
				if !ok || len(services) == 0 {
					t.Fatalf("services = %#v, want non-empty", document["services"])
				}
			},
		},
		{
			name:    "RunSequence",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"holon": "gabriel-greeting-go", "sequence": "multilingual-greeting", "params": map[string]any{"name": "Alice", "lang_code": "fr"}, "dry_run": true})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				resultPayload := payload["result"].(map[string]any)
				steps, ok := resultPayload["steps"].([]any)
				if !ok || len(steps) < 2 {
					t.Fatalf("steps = %#v, want at least 2", resultPayload["steps"])
				}
			},
		},
		{
			name:    "ModInit",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ModRoot },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"holon_path": "alpha-builder"})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if !strings.Contains(payload["modFile"].(string), "holon.mod") {
					t.Fatalf("modFile = %#v, want holon.mod", payload["modFile"])
				}
			},
		},
		{
			name:    "ModAdd",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ModRoot },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				writeInvokeCachedDependencyFixture(t, sb, f.DepPath, f.DepVersion)
				_ = os.WriteFile(filepath.Join(f.ModRoot, "holon.mod"), []byte("holon alpha-builder\n"), 0o644)
			},
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"module": f.DepPath, "version": f.DepVersion})
			},
			assert: func(t *testing.T, result integration.CmdResult, f invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				dep := payload["dependency"].(map[string]any)
				if dep["version"] != f.DepVersion {
					t.Fatalf("version = %#v, want %s", dep["version"], f.DepVersion)
				}
			},
		},
		{
			name:    "ModRemove",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ModRoot },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				writeInvokeModDependency(t, sb, f)
			},
			payload: func(t *testing.T, f invokeOPFixture) string { return mustJSON(t, map[string]any{"module": f.DepPath}) },
			assert: func(t *testing.T, result integration.CmdResult, f invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if payload["path"] != f.DepPath {
					t.Fatalf("path = %#v, want %s", payload["path"], f.DepPath)
				}
			},
		},
		{
			name:    "ModTidy",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ModRoot },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				writeInvokeModDependency(t, sb, f)
				_ = os.WriteFile(filepath.Join(f.ModRoot, "holon.sum"), []byte(f.DepPath+" "+f.DepVersion+" h1:keep\n"+"github.com/example/stale v9.9.9 h1:drop\n"), 0o644)
			},
			payload: func(t *testing.T, f invokeOPFixture) string { return "{}" },
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				pruned, ok := payload["pruned"].([]any)
				if !ok || len(pruned) == 0 {
					t.Fatalf("pruned = %#v, want non-empty", payload["pruned"])
				}
			},
		},
		{
			name:    "ModPull",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ModRoot },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				writeInvokeModDependency(t, sb, f)
			},
			payload: func(t *testing.T, f invokeOPFixture) string { return "{}" },
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				fetched, ok := payload["fetched"].([]any)
				if !ok || len(fetched) == 0 {
					t.Fatalf("fetched = %#v, want non-empty", payload["fetched"])
				}
			},
		},
		{
			name:    "ModUpdate",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ModRoot },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				writeInvokeModDependency(t, sb, f)
			},
			payload: func(t *testing.T, f invokeOPFixture) string { return "{}" },
			assert: func(t *testing.T, result integration.CmdResult, f invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				updated, ok := payload["updated"].([]any)
				if !ok || len(updated) == 0 {
					t.Fatalf("updated = %#v, want non-empty", payload["updated"])
				}
				dep, ok := updated[0].(map[string]any)
				if !ok {
					t.Fatalf("updated[0] = %#v, want object", updated[0])
				}
				if dep["path"] != f.DepPath {
					t.Fatalf("path = %#v, want %s", dep["path"], f.DepPath)
				}
				if dep["oldVersion"] != f.DepVersion {
					t.Fatalf("oldVersion = %#v, want %s", dep["oldVersion"], f.DepVersion)
				}
				if dep["newVersion"] == "" || dep["newVersion"] == dep["oldVersion"] {
					t.Fatalf("newVersion = %#v, want a newer version", dep["newVersion"])
				}
			},
		},
		{
			name:    "ModList",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ModRoot },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				writeInvokeModDependency(t, sb, f)
			},
			payload: func(t *testing.T, f invokeOPFixture) string { return "{}" },
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				deps, ok := payload["dependencies"].([]any)
				if !ok || len(deps) == 0 {
					t.Fatalf("dependencies = %#v, want non-empty", payload["dependencies"])
				}
			},
		},
		{
			name:    "ModGraph",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.ModRoot },
			prepare: func(t *testing.T, sb *integration.Sandbox, transport invokeTransport, f invokeOPFixture) {
				writeInvokeModDependency(t, sb, f)
			},
			payload: func(t *testing.T, f invokeOPFixture) string { return "{}" },
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				edges, ok := payload["edges"].([]any)
				if !ok || len(edges) == 0 {
					t.Fatalf("edges = %#v, want non-empty", payload["edges"])
				}
			},
		},
		{
			name:    "Tools",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string {
				return mustJSON(t, map[string]any{"target": "gabriel-greeting-go", "format": "openai"})
			},
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if payload["format"] != "openai" {
					t.Fatalf("format = %#v, want openai", payload["format"])
				}
				if len(mustBase64Payload(t, payload["payload"])) == 0 {
					t.Fatal("payload was empty")
				}
			},
		},
		{
			name:    "Env",
			workDir: func(t *testing.T, f invokeOPFixture) string { return f.Workspace },
			payload: func(t *testing.T, f invokeOPFixture) string { return mustJSON(t, map[string]any{"shell": true}) },
			assert: func(t *testing.T, result integration.CmdResult, _ invokeOPFixture) {
				t.Helper()
				integration.RequireSuccess(t, result)
				payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
				if payload["shell"] == "" {
					t.Fatalf("shell = %#v, want non-empty", payload["shell"])
				}
			},
		},
	}

	if testing.Short() {
		fixtureCases = fixtureCases[:8]
	}

	for _, transport := range exampleInvokeTransports() {
		transport := transport
		t.Run(transport.Name, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			fixture := newInvokeOPFixture(t, sb)

			for _, tc := range fixtureCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					if tc.prepare != nil {
						tc.prepare(t, sb, transport, fixture)
					}
					workDir := tc.workDir(t, fixture)
					target, opts, cleanup := startOPTarget(t, sb, transport, workDir)
					defer cleanup()
					result := invokeCLIResult(t, sb, opts, target, tc.name, tc.payload(t, fixture))
					tc.assert(t, result, fixture)
				})
			}
		})
	}
}
