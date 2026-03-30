package who

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	openv "github.com/organic-programming/grace-op/internal/env"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/testutil"
)

func TestCreateFromJSONWritesGeneratedByOp(t *testing.T) {
	root := t.TempDir()
	chdirWhoTest(t, root)

	resp, err := CreateFromJSON(`{"given_name":"Megg","family_name":"Prober","motto":"Know what you have.","composer":"B. ALTER","clade":"deterministic/io_bound"}`)
	if err != nil {
		t.Fatalf("CreateFromJSON returned error: %v", err)
	}

	data, err := os.ReadFile(resp.GetFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `option (holons.v1.manifest) = {`) {
		t.Fatalf("manifest missing manifest option: %s", string(data))
	}
	if !strings.Contains(string(data), `given_name: "Megg"`) {
		t.Fatalf("manifest missing given_name: %s", string(data))
	}
	if strings.Contains(string(data), "aliases:") {
		t.Fatalf("manifest unexpectedly contains aliases: %s", string(data))
	}
}

func TestCreateInteractiveUsesOpBannerAndNoAliasesPrompt(t *testing.T) {
	root := t.TempDir()
	chdirWhoTest(t, root)

	input := strings.Join([]string{
		"Prober",
		"Megg",
		"B. ALTER",
		"Know what you have.",
		"3",
		"1",
		"",
		"",
	}, "\n") + "\n"
	var out bytes.Buffer

	resp, err := CreateInteractive(strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("CreateInteractive returned error: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "op new — New Holon Identity") {
		t.Fatalf("interactive output missing op banner: %q", text)
	}
	if strings.Contains(text, "Dummy TestHolon") {
		t.Fatalf("interactive output still mentions Sophia: %q", text)
	}
	if strings.Contains(text, "Aliases") {
		t.Fatalf("interactive output still mentions aliases: %q", text)
	}
	if _, err := os.Stat(resp.GetFilePath()); err != nil {
		t.Fatalf("created holon manifest missing: %v", err)
	}
}

func TestShowResolvesUUIDPrefix(t *testing.T) {
	root := t.TempDir()
	chdirWhoTest(t, root)

	id := identity.New()
	id.GeneratedBy = "op"
	id.GivenName = "Prefix"
	id.FamilyName = "Match"
	id.Motto = "Matches prefix."
	id.Composer = "test"
	id.Clade = "deterministic/pure"
	id.Reproduction = "manual"
	id.Lang = "go"
	id.UUID = "abc12345-0000-0000-0000-000000000000"
	dir := filepath.Join(root, "holons", "prefix-match")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WriteIdentityFile(id, filepath.Join(dir, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}

	resp, err := Show("abc12345")
	if err != nil {
		t.Fatalf("Show returned error: %v", err)
	}
	if resp.GetIdentity().GetUuid() != id.UUID {
		t.Fatalf("uuid = %q, want %q", resp.GetIdentity().GetUuid(), id.UUID)
	}
}

func TestListIncludesLocalAndCachedIdentities(t *testing.T) {
	root := t.TempDir()
	chdirWhoTest(t, root)

	runtimeHome := filepath.Join(root, ".runtime")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OPBIN", filepath.Join(runtimeHome, "bin"))

	localID := identity.New()
	localID.GeneratedBy = "op"
	localID.GivenName = "Local"
	localID.FamilyName = "Holon"
	localID.Motto = "Local."
	localID.Composer = "test"
	localID.Clade = "deterministic/pure"
	localID.Reproduction = "manual"
	localID.Lang = "go"
	localDir := filepath.Join(root, "holons", "local-holon")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WriteIdentityFile(localID, filepath.Join(localDir, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}

	cachedID := identity.New()
	cachedID.GeneratedBy = "op"
	cachedID.GivenName = "Cached"
	cachedID.FamilyName = "Holon"
	cachedID.Motto = "Cached."
	cachedID.Composer = "test"
	cachedID.Clade = "deterministic/pure"
	cachedID.Reproduction = "manual"
	cachedID.Lang = "go"
	cacheDir := filepath.Join(openv.CacheDir(), "github.com/example/cached@v1.0.0")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testutil.WriteIdentityFile(cachedID, filepath.Join(cacheDir, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}

	resp, err := List(root)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(resp.GetEntries()) != 2 {
		t.Fatalf("entries = %d, want 2", len(resp.GetEntries()))
	}

	origins := map[string]string{}
	for _, entry := range resp.GetEntries() {
		origins[entry.GetIdentity().GetGivenName()] = entry.GetOrigin()
	}
	if origins["Local"] != "local" {
		t.Fatalf("Local origin = %q, want local", origins["Local"])
	}
	if origins["Cached"] != "cached" {
		t.Fatalf("Cached origin = %q, want cached", origins["Cached"])
	}
}

func TestListAndShowSupportProtoBackedHolons(t *testing.T) {
	exampleDir := filepath.Join(repoRoot(t), "examples", "hello-world", "gabriel-greeting-go")
	runtimeHome := filepath.Join(t.TempDir(), ".runtime")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OPBIN", filepath.Join(runtimeHome, "bin"))

	resp, err := List(exampleDir)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(resp.GetEntries()) != 1 {
		t.Fatalf("entries = %d, want 1", len(resp.GetEntries()))
	}

	entry := resp.GetEntries()[0]
	if got := entry.GetRelativePath(); got != "." {
		t.Fatalf("relative path = %q, want %q", got, ".")
	}
	if got := entry.GetIdentity().GetGivenName(); got != "Gabriel" {
		t.Fatalf("given_name = %q, want %q", got, "Gabriel")
	}
	if got := entry.GetIdentity().GetFamilyName(); got != "Greeting-Go" {
		t.Fatalf("family_name = %q, want %q", got, "Greeting-Go")
	}

	chdirWhoTest(t, exampleDir)
	shown, err := Show("3f08b5c3")
	if err != nil {
		t.Fatalf("Show returned error: %v", err)
	}
	if filepath.Base(shown.GetFilePath()) != "holon.proto" {
		t.Fatalf("file_path = %q, want holon.proto", shown.GetFilePath())
	}
	if !strings.Contains(shown.GetRawContent(), "option (holons.v1.manifest)") {
		t.Fatalf("raw content missing manifest option: %q", shown.GetRawContent())
	}
}

func chdirWhoTest(t *testing.T, dir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
