package api_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)


func TestGenerateTemplateCreatesScaffold(t *testing.T) {
	root := t.TempDir()

	resp, err := api.GenerateTemplate(&opv1.GenerateTemplateRequest{
		Template: "go-daemon",
		Slug:     "delta-engine",
		Dir:      root,
	})
	if err != nil {
		t.Fatalf("GenerateTemplate error = %v", err)
	}
	if got := filepath.Base(resp.GetDir()); got != "delta-engine" {
		t.Fatalf("dir basename = %q, want %q", got, "delta-engine")
	}
	if _, err := os.Stat(filepath.Join(resp.GetDir(), "cmd", "delta-engine", "main.go")); err != nil {
		t.Fatalf("generated main.go missing: %v", err)
	}
}
