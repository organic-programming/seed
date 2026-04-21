//go:build e2e

package invoke_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

const invokeCommandTimeout = 20 * time.Minute

type invokeTransport struct {
	Name string
}

type invokeOPFixture struct {
	Workspace  string
	ShowUUID   string
	ScratchDir string
	ModRoot    string
	DepPath    string
	DepVersion string
	TestTarget string
}

func invokeCLIResult(t *testing.T, sb *integration.Sandbox, opts integration.RunOptions, target, method, payload string) integration.CmdResult {
	t.Helper()
	if opts.Timeout <= 0 {
		opts.Timeout = invokeCommandTimeout
	}
	args := []string{"--format", "json", "invoke", target, method}
	if payload != "" {
		args = append(args, payload)
	}
	return sb.RunOPWithOptions(t, opts, args...)
}

func invokeCLIJSON(t *testing.T, sb *integration.Sandbox, opts integration.RunOptions, target, method, payload string) map[string]any {
	t.Helper()
	result := invokeCLIResult(t, sb, opts, target, method, payload)
	integration.RequireSuccess(t, result)
	return integration.DecodeJSON[map[string]any](t, result.Stdout)
}

func exampleInvokeTransports() []invokeTransport {
	transports := []invokeTransport{
		{Name: "stdio"},
		{Name: "tcp"},
	}
	if runtime.GOOS != "windows" {
		transports = append(transports, invokeTransport{Name: "unix"})
	}
	return transports
}

func startExampleTransportTarget(t *testing.T, sb *integration.Sandbox, slug string, transport invokeTransport) (string, func()) {
	t.Helper()

	switch transport.Name {
	case "stdio":
		return "stdio://" + slug, func() {}
	case "tcp":
		return "tcp://" + slug, func() {}
	case "unix":
		return "unix://" + slug, func() {}
	default:
		t.Fatalf("unknown transport %q", transport.Name)
		return "", func() {}
	}
}

func newInvokeOPFixture(t *testing.T, sb *integration.Sandbox) invokeOPFixture {
	t.Helper()

	fakeGit := integration.InstallFakeGitLSRemote(t, map[string][]string{
		"https://github.com/example/dep.git": {"v1.0.0", "v1.5.0"},
		"https://github.com/example/dep":     {"v1.0.0", "v1.5.0"},
	})
	t.Setenv("PATH", integration.PathWithPrepend(fakeGit))

	list := integration.ReadListJSON(t, sb)
	if len(list.Entries) == 0 {
		t.Fatal("list returned no entries for invoke fixture")
	}
	testTarget := ""
	for _, spec := range integration.NativeTestHolons(t) {
		if spec.Slug != "gabriel-greeting-go" && integration.SupportsOPTest(spec) {
			testTarget = spec.Slug
			break
		}
	}

	modRoot := t.TempDir()
	writeInvokeModRootFixture(t, modRoot)

	return invokeOPFixture{
		Workspace:  integration.DefaultWorkspaceDir(t),
		ShowUUID:   list.Entries[0].Identity.UUID,
		ScratchDir: t.TempDir(),
		ModRoot:    modRoot,
		DepPath:    "github.com/example/dep",
		DepVersion: "v1.0.0",
		TestTarget: testTarget,
	}
}

func writeInvokeModRootFixture(t *testing.T, root string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, "api", "v1"), 0o755); err != nil {
		t.Fatalf("mkdir mod root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", "v1", "holon.proto"), []byte("syntax = \"proto3\";\npackage sample.v1;\n"), 0o644); err != nil {
		t.Fatalf("write holon.proto: %v", err)
	}
}

func writeInvokeCachedDependencyFixture(t *testing.T, sb *integration.Sandbox, depPath, version string) string {
	t.Helper()

	cacheDir := filepath.Join(sb.OPPATH, "cache", depPath+"@"+version)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	manifest := `syntax = "proto3";
package dep.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "9a8b7c6d-5e4f-4321-8765-abcdefabcdef"
    given_name: "Cached"
    family_name: "Dep"
    motto: "Cached dependency."
    composer: "test"
    status: "draft"
    born: "2026-01-01"
    version: "0.1.0"
    lang: "go"
  }
  description: "Cached dependency fixture."
  lang: "go"
  kind: "native"
  build: { runner: "go-module" main: "./cmd" }
  artifacts: { binary: "cached-dep" }
  contract: { proto: "holon.proto" service: "dep.v1.Dummy" rpcs: ["Ping"] }
};

service Dummy {
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {}
message PingResponse {}
`
	if err := os.WriteFile(filepath.Join(cacheDir, "holon.proto"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write dependency holon.proto: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "holon.mod"), []byte("holon "+depPath+"\n\nrequire (\n    github.com/example/subdep v0.2.0\n)\n"), 0o644); err != nil {
		t.Fatalf("write dependency holon.mod: %v", err)
	}
	return cacheDir
}

func startOPTarget(t *testing.T, sb *integration.Sandbox, transport invokeTransport, workDir string) (string, integration.RunOptions, func()) {
	t.Helper()

	opts := integration.RunOptions{WorkDir: workDir}
	if workDir != integration.DefaultWorkspaceDir(t) {
		opts.SkipDiscoverRoot = true
	}

	switch transport.Name {
	case "stdio":
		return "stdio://op", opts, func() {}
	case "tcp":
		process := sb.StartProcess(t, opts, "serve", "--listen", "tcp://127.0.0.1:0", "--reflect")
		address := process.WaitForListenAddress(t, integration.ProcessStartTimeout)
		return address, opts, func() { process.Stop(t) }
	case "unix":
		socketPath := integration.ShortSocketPath(t, "op")
		process := sb.StartProcess(t, opts, "serve", "--listen", "unix://"+socketPath, "--reflect")
		process.WaitForListenAddress(t, integration.ProcessStartTimeout)
		return "unix://" + socketPath, opts, func() { process.Stop(t) }
	default:
		t.Fatalf("unknown transport %q", transport.Name)
		return "", opts, func() {}
	}
}

func mustBase64Payload(t *testing.T, value any) []byte {
	t.Helper()
	encoded, ok := value.(string)
	if !ok {
		t.Fatalf("payload = %#v, want base64 string", value)
	}
	out, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode base64 payload: %v", err)
	}
	return out
}

func writeInvokeModDependency(t *testing.T, sb *integration.Sandbox, fixture invokeOPFixture) {
	t.Helper()
	writeInvokeCachedDependencyFixture(t, sb, fixture.DepPath, fixture.DepVersion)
	contents := "holon alpha-builder\n\nrequire (\n    " + fixture.DepPath + " " + fixture.DepVersion + "\n)\n"
	if err := os.WriteFile(filepath.Join(fixture.ModRoot, "holon.mod"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write holon.mod: %v", err)
	}
}

func invokeModOptions(fixture invokeOPFixture) integration.RunOptions {
	return integration.RunOptions{SkipDiscoverRoot: true, WorkDir: fixture.ModRoot}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(data)
}

// nonEmptyLines splits s on newlines and returns non-empty, trimmed lines.
// Used to parse JSON Lines output from multi-payload op invoke calls.
func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

