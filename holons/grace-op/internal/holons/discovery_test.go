package holons

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/organic-programming/grace-op/internal/identity"
)

func TestDiscoverHolonsRecursesAndSkipsExcludedDirs(t *testing.T) {
	root := t.TempDir()

	writeDiscoveryHolon(t, filepath.Join(root, "holons", "alpha"), discoveryHolonSeed{
		uuid:       "alpha-uuid",
		givenName:  "Alpha",
		familyName: "Go",
		binaryName: "alpha",
	})
	writeDiscoveryHolon(t, filepath.Join(root, "recipes", "beta"), discoveryHolonSeed{
		uuid:       "beta-uuid",
		givenName:  "Beta",
		familyName: "Rust",
		binaryName: "beta",
	})
	for _, skipped := range []string{
		filepath.Join(root, ".git", "ignored"),
		filepath.Join(root, ".op", "ignored"),
		filepath.Join(root, "node_modules", "ignored"),
		filepath.Join(root, "vendor", "ignored"),
		filepath.Join(root, "build", "ignored"),
		filepath.Join(root, "testdata", "ignored"),
		filepath.Join(root, ".hidden", "ignored"),
	} {
		writeDiscoveryHolon(t, skipped, discoveryHolonSeed{
			uuid:       filepath.Base(skipped) + "-uuid",
			givenName:  "Ignored",
			familyName: "Holon",
			binaryName: "ignored-holon",
		})
	}

	located, err := DiscoverHolons(root)
	if err != nil {
		t.Fatalf("DiscoverHolons returned error: %v", err)
	}
	if len(located) != 2 {
		t.Fatalf("located = %d, want 2", len(located))
	}

	got := make(map[string]string, len(located))
	for _, holon := range located {
		got[holon.Identity.UUID] = filepath.ToSlash(holon.RelativePath)
	}
	if got["alpha-uuid"] != "holons/alpha" {
		t.Fatalf("alpha relative path = %q, want %q", got["alpha-uuid"], "holons/alpha")
	}
	if got["beta-uuid"] != "recipes/beta" {
		t.Fatalf("beta relative path = %q, want %q", got["beta-uuid"], "recipes/beta")
	}
}

func TestDiscoverHolonsDedupsSameUUIDClosestToRoot(t *testing.T) {
	root := t.TempDir()

	writeDiscoveryHolon(t, filepath.Join(root, "rob-go"), discoveryHolonSeed{
		uuid:       "same-uuid",
		givenName:  "Rob",
		familyName: "Go",
		binaryName: "rob-go",
	})
	writeDiscoveryHolon(t, filepath.Join(root, "nested", "rob-go"), discoveryHolonSeed{
		uuid:       "same-uuid",
		givenName:  "Rob",
		familyName: "Go",
		binaryName: "rob-go",
	})

	located, err := DiscoverHolons(root)
	if err != nil {
		t.Fatalf("DiscoverHolons returned error: %v", err)
	}
	if len(located) != 1 {
		t.Fatalf("located = %d, want 1", len(located))
	}
	if got := filepath.Base(located[0].Dir); got != "rob-go" {
		t.Fatalf("dir basename = %q, want %q", got, "rob-go")
	}
	if got := filepath.ToSlash(located[0].RelativePath); got != "rob-go" {
		t.Fatalf("relative path = %q, want %q", got, "rob-go")
	}
}

func TestDiscoverHolonsFindsProtoHolonWithLocalContractImport(t *testing.T) {
	root := t.TempDir()
	writeSharedHolonManifestProto(t, root)

	dir := filepath.Join(root, "grace-op")
	if err := os.MkdirAll(filepath.Join(dir, "api", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	holonProto := `syntax = "proto3";

package op.v1;

import "holons/v1/manifest.proto";

option go_package = "example.com/grace-op/gen/go/op/v1;opv1";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "28f22ab5-c62d-41f8-9ada-e34333060ff9"
    given_name: "Grace"
    family_name: "OP"
    motto: "One command, every holon."
    composer: "B. ALTER"
    status: "draft"
    born: "2026-02-12"
  }
  lang: "go"
  kind: "native"
  build: {
    runner: "go-module"
    main: "./cmd/op"
  }
  requires: {
    commands: ["go"]
    files: ["go.mod"]
  }
  artifacts: {
    binary: "op"
  }
  contract: {
    proto: "api/v1/holon.proto"
    service: "op.v1.OPService"
    rpcs: ["Discover"]
  }
};

service OPService {
  rpc Discover (DiscoverRequest) returns (DiscoverResponse);
}

message DiscoverRequest {}
message DiscoverResponse {}
`
	if err := os.WriteFile(filepath.Join(dir, "api", "v1", "holon.proto"), []byte(holonProto), 0o644); err != nil {
		t.Fatal(err)
	}

	located, err := DiscoverHolons(dir)
	if err != nil {
		t.Fatalf("DiscoverHolons returned error: %v", err)
	}
	if len(located) != 1 {
		t.Fatalf("located = %d, want 1", len(located))
	}
	if got := located[0].Identity.GivenName; got != "Grace" {
		t.Fatalf("given_name = %q, want %q", got, "Grace")
	}
	if got := filepath.Base(located[0].IdentityPath); got != identity.ProtoManifestFileName {
		t.Fatalf("identity path basename = %q, want %q", got, identity.ProtoManifestFileName)
	}
}

func TestResolveTargetRejectsAmbiguousSlugWithDifferentUUIDs(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)

	writeDiscoveryHolon(t, filepath.Join(root, "team-a", "rob-go"), discoveryHolonSeed{
		uuid:       "c7f3a1b2-1111-1111-1111-111111111111",
		givenName:  "Rob",
		familyName: "Go",
		binaryName: "rob-go",
	})
	writeDiscoveryHolon(t, filepath.Join(root, "team-b", "rob-go"), discoveryHolonSeed{
		uuid:       "d8e0f1a2-2222-2222-2222-222222222222",
		givenName:  "Rob",
		familyName: "Go",
		binaryName: "rob-go",
	})

	_, err := ResolveTarget("rob-go")
	if err == nil {
		t.Fatal("expected ambiguous slug error")
	}
	if !strings.Contains(err.Error(), `ambiguous holon "rob-go"`) {
		t.Fatalf("error = %q, want ambiguous holon", err.Error())
	}
	if !strings.Contains(err.Error(), "./team-a/rob-go") || !strings.Contains(err.Error(), "./team-b/rob-go") {
		t.Fatalf("error missing disambiguation paths: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "c7f3a1b2") || !strings.Contains(err.Error(), "d8e0f1a2") {
		t.Fatalf("error missing UUID prefixes: %q", err.Error())
	}
}

func TestResolveTargetUsesShallowestMatchForSameSlugAndUUID(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)

	writeDiscoveryHolon(t, filepath.Join(root, "rob-go"), discoveryHolonSeed{
		uuid:       "same-uuid",
		givenName:  "Rob",
		familyName: "Go",
		binaryName: "rob-go",
	})
	writeDiscoveryHolon(t, filepath.Join(root, "nested", "rob-go"), discoveryHolonSeed{
		uuid:       "same-uuid",
		givenName:  "Rob",
		familyName: "Go",
		binaryName: "rob-go",
	})

	target, err := ResolveTarget("rob-go")
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}
	gotDir, err := filepath.EvalSymlinks(target.Dir)
	if err != nil {
		gotDir = filepath.Clean(target.Dir)
	}
	wantDir, err := filepath.EvalSymlinks(filepath.Join(root, "rob-go"))
	if err != nil {
		wantDir = filepath.Join(root, "rob-go")
	}
	if gotDir != wantDir {
		t.Fatalf("target dir = %q, want %q", gotDir, wantDir)
	}
}

func TestResolveTargetUsesAliasesButNotGivenNames(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)

	dir := filepath.Join(root, "dummy-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	id := identity.Identity{
		UUID:        "1234",
		GivenName:   "Sophia",
		FamilyName:  "TestHolon",
		Motto:       "Know thyself.",
		Composer:    "test",
		Clade:       "deterministic/pure",
		Status:      "draft",
		Born:        "2026-03-06",
		Aliases:     []string{"who"},
		GeneratedBy: "test",
		Lang:        "go",
	}
	writeManifestWithIdentity(t, dir, id, "kind: native\nbuild:\n  runner: go-module\nartifacts:\n  binary: dummy-test\n")

	// Aliases must resolve.
	target, err := ResolveTarget("who")
	if err != nil {
		t.Fatalf("expected alias lookup to succeed, got: %v", err)
	}
	if filepath.Base(target.Dir) != "dummy-test" {
		t.Fatalf("alias resolved to %q, want dummy-test", filepath.Base(target.Dir))
	}

	// Raw given names must still fail.
	if _, err := ResolveTarget("Sophia"); err == nil {
		t.Fatal("expected given-name lookup to fail")
	}
}

func TestDiscoverInOPBINIncludesBundleArtifacts(t *testing.T) {
	root := t.TempDir()
	opbin := filepath.Join(root, "bin")
	t.Setenv("OPPATH", root)
	t.Setenv("OPBIN", opbin)

	bundle := filepath.Join(opbin, "demo.app")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opbin, "demo.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := DiscoverInOPBIN()
	joined := strings.Join(entries, "\n")
	if !strings.Contains(joined, "demo.app -> ") {
		t.Fatalf("DiscoverInOPBIN() missing app bundle: %v", entries)
	}
	if !strings.Contains(joined, "demo.html -> ") {
		t.Fatalf("DiscoverInOPBIN() missing html artifact: %v", entries)
	}
}

func TestResolveInstalledBinaryFindsAppBundleBySlug(t *testing.T) {
	root := t.TempDir()
	opbin := filepath.Join(root, "bin")
	t.Setenv("OPPATH", root)
	t.Setenv("OPBIN", opbin)

	bundle := filepath.Join(opbin, "studio.app")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}

	resolved := ResolveInstalledBinary("studio")
	if resolved != bundle {
		t.Fatalf("ResolveInstalledBinary() = %q, want %q", resolved, bundle)
	}
}

func TestResolveInstalledBinaryFindsHolonPackageBySlug(t *testing.T) {
	root := t.TempDir()
	opbin := filepath.Join(root, "bin")
	t.Setenv("OPPATH", root)
	t.Setenv("OPBIN", opbin)

	packageDir := filepath.Join(opbin, "gabriel-greeting-go.holon")
	binaryPath := filepath.Join(packageDir, "bin", runtimeArchitecture(), "gabriel-greeting-go")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	resolved := ResolveInstalledBinary("gabriel-greeting-go")
	if resolved != binaryPath {
		t.Fatalf("ResolveInstalledBinary() = %q, want %q", resolved, binaryPath)
	}
}

type discoveryHolonSeed struct {
	uuid       string
	givenName  string
	familyName string
	binaryName string
}

func writeDiscoveryHolon(t *testing.T, dir string, seed discoveryHolonSeed) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	id := identity.Identity{
		UUID:        seed.uuid,
		GivenName:   seed.givenName,
		FamilyName:  seed.familyName,
		Motto:       "Test.",
		Composer:    "test",
		Clade:       "deterministic/pure",
		Status:      "draft",
		Born:        "2026-03-07",
		GeneratedBy: "test",
		Lang:        "go",
	}
	writeManifestWithIdentity(t, dir, id, "kind: native\nbuild:\n  runner: go-module\nartifacts:\n  binary: "+seed.binaryName+"\n")
}
