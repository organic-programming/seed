package cli

import (
	"path/filepath"
	"testing"
)

func TestCompletionAmbiguousSlugReturnsAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	first := writeProtoInstallFixture(t, filepath.Join(root, "workspace-a"), "first")
	second := writeProtoInstallFixture(t, filepath.Join(root, "workspace-b"), "second")

	got, _ := completeHolonSlugs(nil, nil, "demo")
	wantFirst := absoluteCompletionPath(first)
	wantSecond := absoluteCompletionPath(second)
	if !completionListContains(got, wantFirst) || !completionListContains(got, wantSecond) {
		t.Fatalf("ambiguous completions = %#v, want absolute paths %q and %q", got, wantFirst, wantSecond)
	}
	if completionListContains(got, "demo-proto") {
		t.Fatalf("ambiguous completions included slug: %#v", got)
	}
}

func TestCompletionUniqueSlugReturnsSlug(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	writeProtoInstallFixture(t, filepath.Join(root, "workspace"), "only")

	got, _ := completeHolonSlugs(nil, nil, "demo")
	if !completionListContains(got, "demo-proto") {
		t.Fatalf("unique completions = %#v, want demo-proto", got)
	}
}

func completionListContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
