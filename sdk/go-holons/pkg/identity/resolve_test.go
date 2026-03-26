package identity_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/organic-programming/go-holons/pkg/identity"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestResolveManifest_ProtoFirst(t *testing.T) {
	dir := filepath.Join(testdataDir(), "protoholon")

	id, source, err := identity.ResolveManifest(dir)
	if err != nil {
		t.Fatalf("ResolveManifest returned error: %v", err)
	}

	if id.UUID != "test-uuid-1234" {
		t.Errorf("UUID = %q, want %q", id.UUID, "test-uuid-1234")
	}
	if id.GivenName != "gabriel" {
		t.Errorf("GivenName = %q, want %q", id.GivenName, "gabriel")
	}
	if id.FamilyName != "Greeting-Go" {
		t.Errorf("FamilyName = %q, want %q", id.FamilyName, "Greeting-Go")
	}
	if id.Motto != "Test greeting holon." {
		t.Errorf("Motto = %q, want %q", id.Motto, "Test greeting holon.")
	}
	if id.Lang != "go" {
		t.Errorf("Lang = %q, want %q", id.Lang, "go")
	}
	if id.Reproduction != "assisted" {
		t.Errorf("Reproduction = %q, want %q", id.Reproduction, "assisted")
	}
	if id.GeneratedBy != "op" {
		t.Errorf("GeneratedBy = %q, want %q", id.GeneratedBy, "op")
	}
	if len(id.Parents) != 1 || id.Parents[0] != "parent-a" {
		t.Errorf("Parents = %v, want [parent-a]", id.Parents)
	}
	if got := id.Slug(); got != "gabriel-greeting-go" {
		t.Errorf("Slug() = %q, want %q", got, "gabriel-greeting-go")
	}
	if !filepath.IsAbs(source) {
		t.Errorf("source should be absolute, got %q", source)
	}
	t.Logf("resolved from: %s", source)
}

func TestResolveManifest_NoManifest(t *testing.T) {
	dir := t.TempDir()

	_, _, err := identity.ResolveManifest(dir)
	if err == nil {
		t.Fatal("ResolveManifest should return error for empty directory")
	}
}
