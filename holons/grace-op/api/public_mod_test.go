package api_test

import (
	"path/filepath"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

func TestModInitAndList(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	initResp, err := api.ModInit(&opv1.ModInitRequest{HolonPath: "sample/alpha"})
	if err != nil {
		t.Fatalf("ModInit error = %v", err)
	}
	if got := filepath.Base(initResp.GetModFile()); got != "holon.mod" {
		t.Fatalf("mod file basename = %q, want %q", got, "holon.mod")
	}
	if got := initResp.GetHolonPath(); got != "sample/alpha" {
		t.Fatalf("holon path = %q, want %q", got, "sample/alpha")
	}

	listResp, err := api.ModList(&opv1.ModListRequest{})
	if err != nil {
		t.Fatalf("ModList error = %v", err)
	}
	if got := listResp.GetHolonPath(); got == "" {
		t.Fatal("holon path must not be empty")
	}
	if len(listResp.GetDependencies()) != 0 {
		t.Fatalf("dependencies = %d, want 0", len(listResp.GetDependencies()))
	}
}
