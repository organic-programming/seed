//go:build !windows

package buildenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNonWindowsBuildEnvironmentIsUnchanged(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "seed-toolchain.yaml"), []byte("malformed: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	env, err := Prepare(root)
	if err != nil || env != nil {
		t.Fatalf("Prepare = %v, %v", env, err)
	}
	cmd, err := Command(root, []string{"missing-build-tool", "argument"}, []string{"TEST=1"})
	want := exec.Command("missing-build-tool", "argument")
	if err != nil || cmd.Path != want.Path || !reflect.DeepEqual(cmd.Args, want.Args) || !reflect.DeepEqual(cmd.Env, append(os.Environ(), "TEST=1")) {
		t.Fatal("non-Windows command changed")
	}
}
