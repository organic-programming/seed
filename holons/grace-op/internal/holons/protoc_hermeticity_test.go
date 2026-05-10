package holons

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConsumerPathHasNoProtocResolution(t *testing.T) {
	repoRoot := graceOpRepoRoot(t)
	var violations []string
	err := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, repoRoot+string(os.PathSeparator)))
		if strings.HasPrefix(rel, "internal/sdkprebuilts/") ||
			strings.HasPrefix(rel, "cmd/protoc-gen-op-adapter/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		needle := "LookPath(" + `"protoc"` + ")"
		if strings.Contains(string(data), needle) {
			violations = append(violations, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("consumer-path protoc LookPath violations: %s", strings.Join(violations, ", "))
	}
}

func graceOpRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
