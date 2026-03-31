package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/organic-programming/go-holons/pkg/identity"
)

func TestDiscoverLocalAllLayers(t *testing.T) {
	root, opHome, opBin := discoverRuntimeFixture(t)

	writePackageHolon(t, filepath.Join(root, "cwd-alpha.holon"), packageSeed{
		slug:       "cwd-alpha",
		uuid:       "uuid-cwd-alpha",
		givenName:  "Cwd",
		familyName: "Alpha",
		entrypoint: "cwd-alpha",
	})
	writePackageHolon(t, filepath.Join(root, ".op", "build", "built-beta.holon"), packageSeed{
		slug:       "built-beta",
		uuid:       "uuid-built-beta",
		givenName:  "Built",
		familyName: "Beta",
		entrypoint: "built-beta",
	})
	writePackageHolon(t, filepath.Join(opBin, "installed-gamma.holon"), packageSeed{
		slug:       "installed-gamma",
		uuid:       "uuid-installed-gamma",
		givenName:  "Installed",
		familyName: "Gamma",
		entrypoint: "installed-gamma",
	})
	writePackageHolon(t, filepath.Join(opHome, "cache", "deps", "cached-delta.holon"), packageSeed{
		slug:       "cached-delta",
		uuid:       "uuid-cached-delta",
		givenName:  "Cached",
		familyName: "Delta",
		entrypoint: "cached-delta",
	})

	result := Discover(LOCAL, nil, &root, ALL, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}

	if got, want := sortedSlugs(result), []string{"built-beta", "cached-delta", "cwd-alpha", "installed-gamma"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverFilterBySpecifiers(t *testing.T) {
	root, _, opBin := discoverRuntimeFixture(t)

	writePackageHolon(t, filepath.Join(root, "cwd-alpha.holon"), packageSeed{slug: "cwd-alpha", uuid: "uuid-cwd-alpha", givenName: "Cwd", familyName: "Alpha", entrypoint: "cwd-alpha"})
	writePackageHolon(t, filepath.Join(root, ".op", "build", "built-beta.holon"), packageSeed{slug: "built-beta", uuid: "uuid-built-beta", givenName: "Built", familyName: "Beta", entrypoint: "built-beta"})
	writePackageHolon(t, filepath.Join(opBin, "installed-gamma.holon"), packageSeed{slug: "installed-gamma", uuid: "uuid-installed-gamma", givenName: "Installed", familyName: "Gamma", entrypoint: "installed-gamma"})

	result := Discover(LOCAL, nil, &root, BUILT|INSTALLED, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}

	if got, want := sortedSlugs(result), []string{"built-beta", "installed-gamma"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverCWDLayerRecurses(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "nested", "alpha.holon"), packageSeed{
		slug:       "alpha",
		uuid:       "uuid-alpha",
		givenName:  "Alpha",
		familyName: "One",
		entrypoint: "alpha",
	})

	result := Discover(LOCAL, nil, &root, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverMatchBySlug(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})
	writePackageHolon(t, filepath.Join(root, "beta.holon"), packageSeed{slug: "beta", uuid: "uuid-beta", givenName: "Beta", familyName: "Two", entrypoint: "beta"})

	expr := "beta"
	result := Discover(LOCAL, &expr, &root, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverMatchByUUIDPrefix(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "12345678-aaaa", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})

	expr := "12345678"
	result := Discover(LOCAL, &expr, &root, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverMatchByAlias(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{
		slug:       "alpha",
		uuid:       "uuid-alpha",
		givenName:  "Alpha",
		familyName: "One",
		entrypoint: "alpha",
		aliases:    []string{"first"},
	})

	expr := "first"
	result := Discover(LOCAL, &expr, &root, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverLimitOne(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})
	writePackageHolon(t, filepath.Join(root, "beta.holon"), packageSeed{slug: "beta", uuid: "uuid-beta", givenName: "Beta", familyName: "Two", entrypoint: "beta"})

	result := Discover(LOCAL, nil, &root, CWD, 1, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got := len(result.Found); got != 1 {
		t.Fatalf("len(found) = %d, want 1", got)
	}
}

func TestDiscoverLimitZeroMeansUnlimited(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})
	writePackageHolon(t, filepath.Join(root, "beta.holon"), packageSeed{slug: "beta", uuid: "uuid-beta", givenName: "Beta", familyName: "Two", entrypoint: "beta"})

	result := Discover(LOCAL, nil, &root, CWD, 0, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got := len(result.Found); got != 2 {
		t.Fatalf("len(found) = %d, want 2", got)
	}
}

func TestDiscoverNegativeLimitReturnsEmpty(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)

	result := Discover(LOCAL, nil, &root, CWD, -1, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got := len(result.Found); got != 0 {
		t.Fatalf("len(found) = %d, want 0", got)
	}
}

func TestDiscoverInvalidSpecifiersReturnsError(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)

	result := Discover(LOCAL, nil, &root, 0xFF, NO_LIMIT, NO_TIMEOUT)
	if result.Error == "" {
		t.Fatal("expected invalid specifiers error")
	}
}

func TestDiscoverSpecifiersZeroTreatedAsAll(t *testing.T) {
	root, opHome, opBin := discoverRuntimeFixture(t)

	writePackageHolon(t, filepath.Join(root, "cwd-alpha.holon"), packageSeed{slug: "cwd-alpha", uuid: "uuid-cwd-alpha", givenName: "Cwd", familyName: "Alpha", entrypoint: "cwd-alpha"})
	writePackageHolon(t, filepath.Join(root, ".op", "build", "built-beta.holon"), packageSeed{slug: "built-beta", uuid: "uuid-built-beta", givenName: "Built", familyName: "Beta", entrypoint: "built-beta"})
	writePackageHolon(t, filepath.Join(opBin, "installed-gamma.holon"), packageSeed{slug: "installed-gamma", uuid: "uuid-installed-gamma", givenName: "Installed", familyName: "Gamma", entrypoint: "installed-gamma"})
	writePackageHolon(t, filepath.Join(opHome, "cache", "deps", "cached-delta.holon"), packageSeed{slug: "cached-delta", uuid: "uuid-cached-delta", givenName: "Cached", familyName: "Delta", entrypoint: "cached-delta"})

	allResult := Discover(LOCAL, nil, &root, ALL, NO_LIMIT, NO_TIMEOUT)
	zeroResult := Discover(LOCAL, nil, &root, 0, NO_LIMIT, NO_TIMEOUT)
	if allResult.Error != "" || zeroResult.Error != "" {
		t.Fatalf("errors = %q / %q", allResult.Error, zeroResult.Error)
	}
	if !reflect.DeepEqual(sortedSlugs(allResult), sortedSlugs(zeroResult)) {
		t.Fatalf("zero specifiers did not match ALL: %v vs %v", sortedSlugs(zeroResult), sortedSlugs(allResult))
	}
}

func TestDiscoverNullExpressionReturnsAll(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})
	writePackageHolon(t, filepath.Join(root, "beta.holon"), packageSeed{slug: "beta", uuid: "uuid-beta", givenName: "Beta", familyName: "Two", entrypoint: "beta"})

	result := Discover(LOCAL, nil, &root, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got := len(result.Found); got != 2 {
		t.Fatalf("len(found) = %d, want 2", got)
	}
}

func TestDiscoverMissingExpressionReturnsEmpty(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})

	expr := "missing"
	result := Discover(LOCAL, &expr, &root, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got := len(result.Found); got != 0 {
		t.Fatalf("len(found) = %d, want 0", got)
	}
}

func TestDiscoverSkipsExcludedDirs(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)

	writeSourceHolon(t, root, filepath.Join(root, "kept"), sourceSeed{uuid: "uuid-kept", givenName: "Kept", familyName: "Holon", binary: "kept"})
	for _, skipped := range []string{
		filepath.Join(root, ".git", "hidden"),
		filepath.Join(root, ".op", "hidden"),
		filepath.Join(root, "node_modules", "hidden"),
		filepath.Join(root, "vendor", "hidden"),
		filepath.Join(root, "build", "hidden"),
		filepath.Join(root, "testdata", "hidden"),
		filepath.Join(root, ".cache", "hidden"),
	} {
		writeSourceHolon(t, root, skipped, sourceSeed{
			uuid:       filepath.Base(skipped) + "-uuid",
			givenName:  "Ignored",
			familyName: "Holon",
			binary:     "ignored",
		})
	}

	result := Discover(LOCAL, nil, &root, SOURCE, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"kept-holon"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverDeduplicatesByUUID(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)

	cwdPath := filepath.Join(root, "alpha.holon")
	builtPath := filepath.Join(root, ".op", "build", "alpha-built.holon")
	writePackageHolon(t, cwdPath, packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})
	writePackageHolon(t, builtPath, packageSeed{slug: "alpha-built", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha-built"})

	result := Discover(LOCAL, nil, &root, ALL, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got := len(result.Found); got != 1 {
		t.Fatalf("len(found) = %d, want 1", got)
	}
	if got, want := result.Found[0].URL, fileURL(cwdPath); got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestDiscoverHolonJSONFastPath(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})

	probeCalls := 0
	SetProbe(func(string) (*HolonEntry, error) {
		probeCalls++
		return nil, os.ErrNotExist
	})
	t.Cleanup(func() { SetProbe(nil) })

	result := Discover(LOCAL, nil, &root, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if probeCalls != 0 {
		t.Fatalf("probeCalls = %d, want 0", probeCalls)
	}
}

func TestDiscoverProbeFallback(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	pkgDir := filepath.Join(root, "alpha.holon")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	probeCalls := 0
	SetProbe(func(packageDir string) (*HolonEntry, error) {
		probeCalls++
		if filepath.Clean(packageDir) != filepath.Clean(pkgDir) {
			return nil, os.ErrNotExist
		}
		entry := &HolonEntry{
			Slug:       "alpha",
			UUID:       "uuid-alpha",
			Dir:        packageDir,
			SourceKind: "package",
			Runner:     "go-module",
			Entrypoint: "alpha",
			Identity: identity.Identity{
				UUID:       "uuid-alpha",
				GivenName:  "Alpha",
				FamilyName: "One",
			},
			Manifest: &Manifest{
				Kind:  "native",
				Build: Build{Runner: "go-module"},
			},
		}
		return entry, nil
	})
	t.Cleanup(func() { SetProbe(nil) })

	result := Discover(LOCAL, nil, &root, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got := len(result.Found); got != 1 {
		t.Fatalf("len(found) = %d, want 1", got)
	}
	if probeCalls != 1 {
		t.Fatalf("probeCalls = %d, want 1", probeCalls)
	}
}

func TestDiscoverSiblingsLayer(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	appExecutable := filepath.Join(root, "TestApp.app", "Contents", "MacOS", "TestApp")
	bundleRoot := filepath.Join(root, "TestApp.app", "Contents", "Resources", "Holons")

	if err := os.MkdirAll(filepath.Dir(appExecutable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appExecutable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePackageHolon(t, filepath.Join(bundleRoot, "bundle.holon"), packageSeed{slug: "bundle", uuid: "uuid-bundle", givenName: "Bundle", familyName: "Holon", entrypoint: "bundle"})

	originalExecutablePath := executablePath
	executablePath = func() (string, error) { return appExecutable, nil }
	t.Cleanup(func() { executablePath = originalExecutablePath })

	result := Discover(LOCAL, nil, &root, SIBLINGS, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"bundle"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverSourceLayer(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writeSourceHolon(t, root, filepath.Join(root, "proto-holon"), sourceSeed{uuid: "uuid-proto", givenName: "Proto", familyName: "Holon", binary: "proto-holon"})

	result := Discover(LOCAL, nil, &root, SOURCE, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"proto-holon"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverBuiltLayer(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, ".op", "build", "built.holon"), packageSeed{slug: "built", uuid: "uuid-built", givenName: "Built", familyName: "Holon", entrypoint: "built"})

	result := Discover(LOCAL, nil, &root, BUILT, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"built"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverInstalledLayer(t *testing.T) {
	root, _, opBin := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(opBin, "installed.holon"), packageSeed{slug: "installed", uuid: "uuid-installed", givenName: "Installed", familyName: "Holon", entrypoint: "installed"})

	result := Discover(LOCAL, nil, &root, INSTALLED, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"installed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverCachedLayer(t *testing.T) {
	root, opHome, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(opHome, "cache", "deep", "cached.holon"), packageSeed{slug: "cached", uuid: "uuid-cached", givenName: "Cached", familyName: "Holon", entrypoint: "cached"})

	result := Discover(LOCAL, nil, &root, CACHED, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"cached"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverNilRootDefaultsToCwd(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})
	t.Chdir(root)

	result := Discover(LOCAL, nil, nil, CWD, NO_LIMIT, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error = %q", result.Error)
	}
	if got, want := sortedSlugs(result), []string{"alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs = %v, want %v", got, want)
	}
}

func TestDiscoverEmptyRootReturnsError(t *testing.T) {
	empty := ""
	result := Discover(LOCAL, nil, &empty, ALL, NO_LIMIT, NO_TIMEOUT)
	if result.Error == "" {
		t.Fatal("expected empty root error")
	}
}

func TestDiscoverUnsupportedScopeReturnsError(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)

	if result := Discover(PROXY, nil, &root, ALL, NO_LIMIT, NO_TIMEOUT); result.Error == "" {
		t.Fatal("expected PROXY error")
	}
	if result := Discover(DELEGATED, nil, &root, ALL, NO_LIMIT, NO_TIMEOUT); result.Error == "" {
		t.Fatal("expected DELEGATED error")
	}
}

func TestResolveKnownSlug(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)
	writePackageHolon(t, filepath.Join(root, "alpha.holon"), packageSeed{slug: "alpha", uuid: "uuid-alpha", givenName: "Alpha", familyName: "One", entrypoint: "alpha"})

	result := Resolve(LOCAL, "alpha", &root, CWD, NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Resolve error = %q", result.Error)
	}
	if result.Ref == nil || result.Ref.Info == nil || result.Ref.Info.Slug != "alpha" {
		t.Fatalf("Resolve ref = %#v", result.Ref)
	}
}

func TestResolveMissing(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)

	result := Resolve(LOCAL, "missing", &root, ALL, NO_TIMEOUT)
	if result.Error == "" {
		t.Fatal("expected missing resolve error")
	}
}

func TestResolveInvalidSpecifiers(t *testing.T) {
	root, _, _ := discoverRuntimeFixture(t)

	result := Resolve(LOCAL, "alpha", &root, 0xFF, NO_TIMEOUT)
	if result.Error == "" {
		t.Fatal("expected invalid specifiers error")
	}
}

type packageSeed struct {
	slug          string
	uuid          string
	givenName     string
	familyName    string
	runner        string
	entrypoint    string
	kind          string
	architectures []string
	hasDist       bool
	hasSource     bool
	aliases       []string
}

type sourceSeed struct {
	uuid       string
	givenName  string
	familyName string
	binary     string
	buildMain  string
	aliases    []string
}

func discoverRuntimeFixture(t *testing.T) (string, string, string) {
	t.Helper()

	root := t.TempDir()
	opHome := filepath.Join(root, "runtime")
	opBin := filepath.Join(opHome, "bin")
	t.Setenv("OPPATH", opHome)
	t.Setenv("OPBIN", opBin)
	return root, opHome, opBin
}

func sortedSlugs(result DiscoverResult) []string {
	slugs := make([]string, 0, len(result.Found))
	for _, ref := range result.Found {
		if ref.Info == nil {
			continue
		}
		slugs = append(slugs, ref.Info.Slug)
	}
	sort.Strings(slugs)
	return slugs
}

func writePackageHolon(t *testing.T, dir string, seed packageSeed) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if seed.slug == "" {
		seed.slug = strings.ToLower(seed.givenName + "-" + seed.familyName)
	}
	if seed.runner == "" {
		seed.runner = "go-module"
	}
	if seed.kind == "" {
		seed.kind = "native"
	}

	architectures := "[]"
	if len(seed.architectures) > 0 {
		architectures = "[" + strings.Join(quoteStrings(seed.architectures), ", ") + "]"
	}
	aliases := "[]"
	if len(seed.aliases) > 0 {
		aliases = "[" + strings.Join(quoteStrings(seed.aliases), ", ") + "]"
	}

	data := fmt.Sprintf(`{
  "schema": "holon-package/v1",
  "slug": %q,
  "uuid": %q,
  "identity": {
    "given_name": %q,
    "family_name": %q,
    "aliases": %s
  },
  "lang": "go",
  "runner": %q,
  "status": "draft",
  "kind": %q,
  "entrypoint": %q,
  "architectures": %s,
  "has_dist": %t,
  "has_source": %t
}
`, seed.slug, seed.uuid, seed.givenName, seed.familyName, aliases, seed.runner, seed.kind, seed.entrypoint, architectures, seed.hasDist, seed.hasSource)

	if err := os.WriteFile(filepath.Join(dir, ".holon.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSourceHolon(t *testing.T, root string, dir string, seed sourceSeed) {
	t.Helper()

	writeSharedManifestProto(t, root)

	if err := os.MkdirAll(filepath.Join(dir, "v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	aliasesBlock := ""
	if len(seed.aliases) > 0 {
		aliasesBlock = fmt.Sprintf("    aliases: [%s]\n", strings.Join(quoteStrings(seed.aliases), ", "))
	}
	buildBlock := "  build: {\n    runner: \"go-module\"\n  }\n"
	if seed.buildMain != "" {
		buildBlock = fmt.Sprintf("  build: {\n    runner: \"go-module\"\n    main: %q\n  }\n", seed.buildMain)
	}

	data := fmt.Sprintf(`syntax = "proto3";

package discover.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: %q
    given_name: %q
    family_name: %q
%s  }
  kind: "native"
  lang: "go"
%s  artifacts: {
    binary: %q
  }
};
`, seed.uuid, seed.givenName, seed.familyName, aliasesBlock, buildBlock, seed.binary)

	if err := os.WriteFile(filepath.Join(dir, "v1", "holon.proto"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func quoteStrings(values []string) []string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return quoted
}

func writeSharedManifestProto(t *testing.T, root string) {
	t.Helper()

	source := filepath.Join(identityTestdataRoot(t), "_protos", "holons", "v1", "manifest.proto")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}

	target := filepath.Join(root, "_protos", "holons", "v1")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", target, err)
	}
	if err := os.WriteFile(filepath.Join(target, "manifest.proto"), data, 0o644); err != nil {
		t.Fatalf("write manifest.proto: %v", err)
	}
}

func identityTestdataRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "identity", "testdata")
}
