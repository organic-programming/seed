package git

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectRefs(t *testing.T) {
	t.Parallel()

	submodules := []string{
		filepath.Join("/repo", "go-holons"),
		filepath.Join("/repo", "rust-holons"),
	}
	taskContent := `# TASK

See github.com/organic-programming/go-holons for context.

## Repository
- rust-holons
`

	got := DetectRefs(taskContent, submodules)
	want := []string{
		filepath.Join("/repo", "go-holons"),
		filepath.Join("/repo", "rust-holons"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectRefs = %v, want %v", got, want)
	}
}
