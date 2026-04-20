package holons

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	"github.com/organic-programming/grace-op/internal/identity"
)

func TestDiscoverHolonsWrapperDelegatesSDK(t *testing.T) {
	root := t.TempDir()
	writeDiscoveryHolon(t, filepath.Join(root, "holons", "alpha"), discoveryHolonSeed{
		uuid:       "alpha-uuid",
		givenName:  "Alpha",
		familyName: "Go",
		binaryName: "alpha",
	})

	expected := sdkdiscover.Discover(sdkdiscover.LOCAL, nil, &root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if expected.Error != "" {
		t.Fatalf("sdk discover returned error: %s", expected.Error)
	}

	located, err := DiscoverHolonsWithOptions(&root, sdkdiscover.SOURCE, sdkdiscover.NO_LIMIT, sdkdiscover.NO_TIMEOUT)
	if err != nil {
		t.Fatalf("DiscoverHolons returned error: %v", err)
	}
	if len(located) != len(expected.Found) {
		t.Fatalf("located = %d, want %d", len(located), len(expected.Found))
	}
	if located[0].Identity.UUID != "alpha-uuid" {
		t.Fatalf("uuid = %q, want %q", located[0].Identity.UUID, "alpha-uuid")
	}
}

func TestResolveTargetWrapperDelegatesSDK(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)
	writeDiscoveryHolon(t, filepath.Join(root, "rob-go"), discoveryHolonSeed{
		uuid:       "same-uuid",
		givenName:  "Rob",
		familyName: "Go",
		binaryName: "rob-go",
	})

	expected := sdkdiscover.Resolve(sdkdiscover.LOCAL, "rob-go", &root, sdkdiscover.ALL, sdkdiscover.NO_TIMEOUT)
	if expected.Error != "" {
		t.Fatalf("sdk resolve returned error: %s", expected.Error)
	}

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
		wantDir = filepath.Clean(filepath.Join(root, "rob-go"))
	}
	if gotDir != wantDir {
		t.Fatalf("target dir = %q, want %q", gotDir, wantDir)
	}
	if target.Identity == nil || target.Identity.UUID != expected.Ref.Info.UUID {
		t.Fatalf("target identity = %+v, want UUID %q", target.Identity, expected.Ref.Info.UUID)
	}
}

func TestTargetFromRefConversion(t *testing.T) {
	root := t.TempDir()
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
	binaryPath := filepath.Join(dir, ".op", "build", filepath.Base(dir)+".holon", "bin", runtimeArchitecture(), "dummy-test")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	ref := &sdkdiscover.HolonRef{
		URL: "file://" + filepath.ToSlash(dir),
		Info: &sdkdiscover.HolonInfo{
			Slug:   "dummy-test",
			UUID:   id.UUID,
			Lang:   id.Lang,
			Status: id.Status,
			Identity: sdkdiscover.IdentityInfo{
				GivenName:  id.GivenName,
				FamilyName: id.FamilyName,
				Motto:      id.Motto,
				Aliases:    append([]string(nil), id.Aliases...),
			},
			SourceKind: "source",
		},
	}

	target, err := targetFromRef(ref)
	if err != nil {
		t.Fatalf("targetFromRef returned error: %v", err)
	}
	if target.Ref != "dummy-test" {
		t.Fatalf("Ref = %q, want %q", target.Ref, "dummy-test")
	}
	if target.Identity == nil || target.Identity.UUID != id.UUID {
		t.Fatalf("Identity = %+v, want UUID %q", target.Identity, id.UUID)
	}
	if target.Manifest == nil {
		t.Fatal("expected manifest to be loaded")
	}
}

func TestResolveBinaryStillWorks(t *testing.T) {
	root := t.TempDir()
	chdirForHolonTest(t, root)
	writeDiscoveryHolon(t, filepath.Join(root, "dummy-test"), discoveryHolonSeed{
		uuid:       "binary-uuid",
		givenName:  "Dummy",
		familyName: "Test",
		binaryName: "dummy-test",
	})

	binaryPath, err := ResolveBinary("dummy-test")
	if err != nil {
		t.Fatalf("ResolveBinary returned error: %v", err)
	}
	if !strings.HasSuffix(binaryPath, filepath.Join("bin", runtime.GOOS+"_"+runtime.GOARCH, "dummy-test")) {
		t.Fatalf("binary path = %q, want built artifact", binaryPath)
	}
}

func TestDiscoverInOPBINStillWorks(t *testing.T) {
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
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific")
	}
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
	binaryPath := filepath.Join(dir, ".op", "build", filepath.Base(dir)+".holon", "bin", runtimeArchitecture(), seed.binaryName)
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
