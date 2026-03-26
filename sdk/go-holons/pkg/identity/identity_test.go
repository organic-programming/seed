package identity_test

import (
	"path/filepath"
	"testing"

	"github.com/organic-programming/go-holons/pkg/identity"
)

func TestResolveProtoFileProvidesSlugIdentity(t *testing.T) {
	path := filepath.Join(testdataDir(), "protoholon", "v1", "holon.proto")

	resolved, err := identity.ResolveProtoFile(path)
	if err != nil {
		t.Fatalf("ResolveProtoFile returned error: %v", err)
	}
	if resolved.Identity.UUID != "test-uuid-1234" {
		t.Fatalf("UUID = %q, want %q", resolved.Identity.UUID, "test-uuid-1234")
	}
	if resolved.Identity.GivenName != "gabriel" {
		t.Fatalf("GivenName = %q, want %q", resolved.Identity.GivenName, "gabriel")
	}
	if got := resolved.Identity.Slug(); got != "gabriel-greeting-go" {
		t.Fatalf("Slug() = %q, want %q", got, "gabriel-greeting-go")
	}
}

func TestSlugTrimsQuestionMark(t *testing.T) {
	id := identity.Identity{
		GivenName:  "Rob",
		FamilyName: "Go?",
	}

	if got := id.Slug(); got != "rob-go" {
		t.Fatalf("Slug() = %q, want %q", got, "rob-go")
	}
}
