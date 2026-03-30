package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsUseHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPPATH", "")
	t.Setenv("OPBIN", "")

	if got, want := OPPATH(), filepath.Join(home, ".op"); got != want {
		t.Fatalf("OPPATH() = %q, want %q", got, want)
	}
	if got, want := OPBIN(), filepath.Join(home, ".op", "bin"); got != want {
		t.Fatalf("OPBIN() = %q, want %q", got, want)
	}
	if got, want := CacheDir(), filepath.Join(home, ".op", "cache"); got != want {
		t.Fatalf("CacheDir() = %q, want %q", got, want)
	}
}

func TestEnvOverridesAreResolved(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OPPATH", filepath.Join(root, "runtime"))
	t.Setenv("OPBIN", filepath.Join(root, "runtime-bin"))

	if got, want := OPPATH(), filepath.Join(root, "runtime"); got != want {
		t.Fatalf("OPPATH() = %q, want %q", got, want)
	}
	if got, want := OPBIN(), filepath.Join(root, "runtime-bin"); got != want {
		t.Fatalf("OPBIN() = %q, want %q", got, want)
	}
	if got, want := CacheDir(), filepath.Join(root, "runtime", "cache"); got != want {
		t.Fatalf("CacheDir() = %q, want %q", got, want)
	}
}

func TestInitCreatesRuntimeDirectories(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OPPATH", filepath.Join(root, "runtime"))
	t.Setenv("OPBIN", filepath.Join(root, "runtime", "bin"))

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	for _, dir := range []string{OPPATH(), OPBIN(), CacheDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("missing %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}
}

func TestShellSnippetContainsExpectedExports(t *testing.T) {
	snippet := ShellSnippet()
	for _, expected := range []string{
		`export OPPATH="${OPPATH:-$HOME/.op}"`,
		`export OPBIN="${OPBIN:-$OPPATH/bin}"`,
		`mkdir -p "$OPBIN"`,
		`export PATH="$OPBIN:$PATH"`,
	} {
		if !strings.Contains(snippet, expected) {
			t.Fatalf("ShellSnippet() missing %q in %q", expected, snippet)
		}
	}
}

func TestRootReturnsAbsoluteCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	got, err := filepath.EvalSymlinks(Root())
	if err != nil {
		got = Root()
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		want = root
	}
	if got != want {
		t.Fatalf("Root() = %q, want %q", got, want)
	}
}
