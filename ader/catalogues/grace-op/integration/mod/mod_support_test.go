//go:build e2e

package mod_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func writeModRootFixture(t *testing.T, root string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, "api", "v1"), 0o755); err != nil {
		t.Fatalf("mkdir mod root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", "v1", "holon.proto"), []byte("syntax = \"proto3\";\npackage sample.v1;\n"), 0o644); err != nil {
		t.Fatalf("write holon.proto: %v", err)
	}
}

func writeCachedDependencyFixture(t *testing.T, sb *integration.Sandbox, depPath, version string) string {
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

func installModRemoteTagsFixture(t *testing.T) {
	t.Helper()
	fakeGit := integration.InstallFakeGitLSRemote(t, map[string][]string{
		"https://github.com/example/dep.git": {"v1.0.0", "v1.5.0"},
		"https://github.com/example/dep":     {"v1.0.0", "v1.5.0"},
	})
	t.Setenv("PATH", integration.PathWithPrepend(fakeGit))
}
