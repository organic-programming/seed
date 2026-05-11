package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedReleaseBumpCommands(t *testing.T) {
	out := runScript(t, "seed_release_bump.go", "next-patch", "1.4.5")
	if strings.TrimSpace(out) != "1.4.6" {
		t.Fatalf("next-patch = %q", out)
	}
	path := filepath.Join(t.TempDir(), "seed-toolchain.yaml")
	if err := os.WriteFile(path, []byte("seed_release: \"0.7.0\"\nother: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out = runScript(t, "seed_release_bump.go", "bump-file", path)
	if strings.TrimSpace(out) != "current=0.7.0\nnext=0.7.1" {
		t.Fatalf("bump-file output = %q", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "seed_release: \"0.7.1\"\nother: true\n" {
		t.Fatalf("bumped file = %q", string(data))
	}
}

func runScript(t *testing.T, script string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"run", "./" + script}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(cmdArgs, " "), err, string(out))
	}
	return string(out)
}
