package buildenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("OP_BUILDENV_NATIVE_CHILD") == "1" {
		executable, err := os.Executable()
		if err != nil {
			os.Exit(2)
		}
		marker, err := os.ReadFile(filepath.Join(filepath.Dir(executable), "selected.txt"))
		if err != nil {
			os.Exit(3)
		}
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(4)
		}
		fmt.Printf("%s|%s", marker, filepath.Base(cwd))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func writeFixture(t *testing.T, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func environmentFixture(t *testing.T, marker string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project é space")
	writeFixture(t, filepath.Join(root, "tools", "private-probe.cmd"), "@echo off\r\necho %PRIVATE_BUILD_VALUE%\r\n")
	writeFixture(t, filepath.Join(root, "seed-toolchain.yaml"), fmt.Sprintf(`build_environments:
  windows:
    path: ["${CONFIG_DIR}/tools"]
    env:
      PRIVATE_BUILD_VALUE: %q
      FROM_ANOTHER_VALUE: "${PRIVATE_BUILD_VALUE}"
      FROM_CONFIG: "${CONFIG_DIR}/tools/private-probe.cmd"
    required_files: ["${FROM_CONFIG}"]
`, marker))
	writeFixture(t, filepath.Join(root, "seed", "seed-toolchain.yaml"), "protoc:\n  version: '32.0'\n")
	if err := os.MkdirAll(filepath.Join(root, "seed", "holons", "probe"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestPrepareWindowsPrivateEnvironment(t *testing.T) {
	root := environmentFixture(t, "private-value")
	before := os.Environ()
	child := filepath.Join(root, "seed", "holons", "probe")
	env, err := Prepare(child)
	if err != nil {
		t.Fatal(err)
	}
	if Value(env, "CONFIG_DIR") != root || Value(env, "PROTOC_VERSION") != "32.0" || Value(env, "FROM_ANOTHER_VALUE") != "private-value" {
		t.Fatal("incorrect ancestor/value resolution")
	}
	path, err := Lookup(env, "private-probe")
	if err != nil || !strings.EqualFold(path, filepath.Join(root, "tools", "private-probe.cmd")) {
		t.Fatalf("private lookup: %q, %v", path, err)
	}
	if _, err := exec.LookPath("private-probe"); err == nil {
		t.Fatal("fixture command leaked into global PATH")
	}
	cmd, err := Command(child, []string{"private-probe"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "private-value" {
		t.Fatalf("child: %v, %s", err, output)
	}
	if !reflect.DeepEqual(before, os.Environ()) {
		t.Fatal("preparation changed process environment")
	}
	// The origin environment remains available to runners using scratch dirs.
	cmd, err = Command(t.TempDir(), []string{"private-probe"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "private-value" {
		t.Fatalf("scratch child: %v, %s", err, output)
	}
}

func TestPrepareWindowsConcurrentProjects(t *testing.T) {
	roots := []string{environmentFixture(t, "one"), environmentFixture(t, "two")}
	before := os.Environ()
	var wg sync.WaitGroup
	for i, root := range roots {
		wg.Add(1)
		go func(i int, root string) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				cmd, err := Command(root, []string{"private-probe"}, nil)
				if err != nil {
					t.Error(err)
					return
				}
				output, err := cmd.CombinedOutput()
				want := []string{"one", "two"}[i]
				if err != nil || strings.TrimSpace(string(output)) != want {
					t.Errorf("project %d: %v, %s", i, err, output)
				}
			}
		}(i, root)
	}
	wg.Wait()
	if !reflect.DeepEqual(before, os.Environ()) {
		t.Fatal("concurrent preparation changed process environment")
	}
}

func TestPrepareWindowsNoConfiguration(t *testing.T) {
	root := t.TempDir()
	env, err := Prepare(root)
	if err != nil || env != nil {
		t.Fatalf("without config = %v, %v", env, err)
	}
	cmd, err := Command(root, []string{"missing-build-tool", "arg"}, nil)
	want := exec.Command("missing-build-tool", "arg")
	if err != nil || cmd.Path != want.Path || cmd.Env != nil || !reflect.DeepEqual(cmd.Args, want.Args) || fmt.Sprint(cmd.Err) != fmt.Sprint(want.Err) {
		t.Fatal("no-config command behavior changed")
	}
}

func TestWindowsCommandResolvesRelativePathsAtChildDirectory(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "child é space")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	native, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct{ dir, marker string }{{parent, "parent"}, {child, "child"}} {
		tools := filepath.Join(fixture.dir, "tools")
		writeFixture(t, filepath.Join(tools, "selected.txt"), fixture.marker)
		if err := os.WriteFile(filepath.Join(tools, "probe.exe"), native, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture(t, filepath.Join(parent, "seed-toolchain.yaml"), "build_environments:\n  windows: {}\n")
	t.Chdir(parent)
	for _, tc := range []struct{ name, dir, command string }{
		{"explicit relative executable", child, "./tools/probe.exe"},
		{"relative PATH", child, "probe"},
		{"relative child directory", "child é space", "probe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := Command(tc.dir, []string{tc.command}, []string{"PATH=tools", "OP_BUILDENV_NATIVE_CHILD=1"})
			if err != nil {
				t.Fatal(err)
			}
			output, err := cmd.CombinedOutput()
			if err != nil || string(output) != "child|child é space" {
				t.Fatalf("child command: %v, %s", err, output)
			}
			if !strings.EqualFold(Value(cmd.Env, "PATH"), filepath.Join(child, "tools")) {
				t.Fatalf("relative child PATH was not anchored: %q", Value(cmd.Env, "PATH"))
			}
		})
	}
	if err := os.Remove(filepath.Join(child, "tools", "probe.exe")); err != nil {
		t.Fatal(err)
	}
	if _, err := Command(child, []string{"./tools/probe.exe"}, []string{"PATH=tools", "OP_BUILDENV_NATIVE_CHILD=1"}); err == nil {
		t.Fatal("missing child executable resolved to parent's homonym")
	}
}

func TestPrepareWindowsInvalidDeclarations(t *testing.T) {
	for _, body := range []string{
		"env: {X: '${DOES_NOT_EXIST_OP_TEST}'}",
		"env: {X: '${Y}', Y: '${X}'}",
		"path: ['missing-folder']",
		"required_files: ['missing-file']",
		"env: {PATH: anything}",
	} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, filepath.Join(root, "seed-toolchain.yaml"), "build_environments:\n  windows:\n    "+body+"\n")
			if _, err := Prepare(root); err == nil {
				t.Fatal("invalid declaration accepted")
			}
		})
	}
}

func TestPrepareWindowsInstalledToolchain(t *testing.T) {
	root := os.Getenv("OP_BUILDENV_TEST_PROJECT")
	if root == "" {
		t.Skip("set OP_BUILDENV_TEST_PROJECT for installed toolchain qualification")
	}
	before := os.Environ()
	env, err := Prepare(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"cl", "link", "cmake", "ninja", "protoc", "dart", "flutter"} {
		path, err := Lookup(env, tool)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %s", tool, path)
	}
	for _, key := range []string{"VSINSTALLDIR", "VCToolsVersion", "WindowsSDKVersion", "CMAKE_GENERATOR", "PROTOC", "LIBCLANG_PATH", "VULKAN_SDK"} {
		t.Logf("%s: %s", key, Value(env, key))
	}
	for _, args := range [][]string{{"ninja", "--version"}, {"protoc", "--version"}, {"dart", "--version"}} {
		cmd, err := Command(root, args, nil)
		if err != nil {
			t.Fatal(err)
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("tool %s: %v", args[0], err)
		}
		t.Log(strings.TrimSpace(string(output)))
	}
	scratch := t.TempDir()
	writeFixture(t, filepath.Join(scratch, "probe.cpp"), "#include <iostream>\nint main() { std::cout << \"native-toolchain-ok\"; }\n")
	cmd, err := Command(scratch, []string{"cl.exe", "/nologo", "/EHsc", "/MD", "probe.cpp", "/Fe:probe.exe"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native C++ compile/link: %v\n%s", err, output)
	}
	output, err := exec.Command(filepath.Join(scratch, "probe.exe")).CombinedOutput()
	if err != nil || string(output) != "native-toolchain-ok" {
		t.Fatalf("native C++ execution: %v %s", err, output)
	}
	t.Log("native C++ compile, link and execution passed with private developer environment")
	if !reflect.DeepEqual(before, os.Environ()) {
		t.Fatal("Visual Studio preparation changed process environment")
	}
}
