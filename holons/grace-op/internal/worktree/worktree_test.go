package worktree

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCreateIsolatedBootstrapsRuntimeAndConfigs(t *testing.T) {
	repo := setupGitRepo(t)
	chdir(t, repo)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", os.Getenv("PATH"))
	homeOpbin := filepath.Join(home, ".op", "bin")
	must(t, os.MkdirAll(homeOpbin, 0o755))
	must(t, os.WriteFile(filepath.Join(homeOpbin, "op"), []byte("home op"), 0o755))
	must(t, os.WriteFile(filepath.Join(homeOpbin, "ader"), []byte("home ader"), 0o755))
	restore, calls := fakeBuildCommand(t)
	defer restore()

	result, err := Create("feature/X", ModeIsolated)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "built" {
		t.Fatalf("status = %q, want built", result.Status)
	}
	worktree := result.Worktree
	for _, path := range []string{
		filepath.Join(worktree, ".op", "bin", "op"),
		filepath.Join(worktree, ".op", "bin", "ader"),
		markerPath(worktree),
		filepath.Join(worktree, ".codex", "config.toml"),
		filepath.Join(worktree, ".claude", "settings.local.json"),
		filepath.Join(worktree, ".gemini", ".env"),
		filepath.Join(worktree, ".vscode", "settings.json"),
	} {
		if !fileExists(path) {
			t.Fatalf("expected generated path missing: %s", path)
		}
	}
	if got := strings.Join(*calls, ","); got != "op,clem-ader" {
		t.Fatalf("build calls = %q, want op,clem-ader", got)
	}
	if got := string(mustRead(t, filepath.Join(homeOpbin, "op"))); got != "home op" {
		t.Fatalf("home op mutated: %q", got)
	}
	if got := string(mustRead(t, filepath.Join(homeOpbin, "ader"))); got != "home ader" {
		t.Fatalf("home ader mutated: %q", got)
	}
}

func TestBootstrapReusesValidMarkerWithoutRebuild(t *testing.T) {
	repo := setupGitRepo(t)
	chdir(t, repo)
	restore, calls := fakeBuildCommand(t)
	defer restore()

	first, err := Create("feature/reuse", ModeIsolated)
	if err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 {
		t.Fatalf("first build calls = %d, want 2", len(*calls))
	}
	second, err := Bootstrap("feature/reuse")
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "reused" {
		t.Fatalf("second status = %q, want reused", second.Status)
	}
	if second.Worktree != first.Worktree {
		t.Fatalf("worktree changed: %s -> %s", first.Worktree, second.Worktree)
	}
	if len(*calls) != 2 {
		t.Fatalf("rerun rebuilt; calls = %v", *calls)
	}
}

func TestPlainThenBootstrapPromotesExistingWorktree(t *testing.T) {
	repo := setupGitRepo(t)
	chdir(t, repo)
	restore, _ := fakeBuildCommand(t)
	defer restore()

	plain, err := Create("feature/plain", ModePlain)
	if err != nil {
		t.Fatal(err)
	}
	if fileExists(markerPath(plain.Worktree)) {
		t.Fatal("plain create wrote bootstrap marker")
	}
	isolated, err := Bootstrap("feature/plain")
	if err != nil {
		t.Fatal(err)
	}
	if isolated.Status != "built" {
		t.Fatalf("bootstrap status = %q, want built", isolated.Status)
	}
	if isolated.Worktree != plain.Worktree {
		t.Fatalf("worktree changed: %s -> %s", plain.Worktree, isolated.Worktree)
	}
	if !fileExists(markerPath(plain.Worktree)) {
		t.Fatal("bootstrap marker missing after promotion")
	}
}

func TestBootstrapPreservesUserConfigKeys(t *testing.T) {
	repo := setupGitRepo(t)
	chdir(t, repo)
	restore, _ := fakeBuildCommand(t)
	defer restore()

	plain, err := Create("feature/config", ModePlain)
	if err != nil {
		t.Fatal(err)
	}
	claudePath := filepath.Join(plain.Worktree, ".claude", "settings.local.json")
	must(t, os.MkdirAll(filepath.Dir(claudePath), 0o755))
	must(t, os.WriteFile(claudePath, []byte(`{"permissions":{"allow":["Bash"]},"env":{"USER_KEY":"1"}}`), 0o644))
	vscodePath := filepath.Join(plain.Worktree, ".vscode", "settings.json")
	must(t, os.MkdirAll(filepath.Dir(vscodePath), 0o755))
	must(t, os.WriteFile(vscodePath, []byte(`{"editor.fontSize":14}`), 0o644))

	if _, err := Bootstrap("feature/config"); err != nil {
		t.Fatal(err)
	}

	var claude map[string]any
	must(t, json.Unmarshal(mustRead(t, claudePath), &claude))
	env := claude["env"].(map[string]any)
	if env["USER_KEY"] != "1" {
		t.Fatalf("USER_KEY not preserved: %#v", env)
	}
	if env["OPPATH"] != filepath.Join(plain.Worktree, ".op") {
		t.Fatalf("OPPATH = %v", env["OPPATH"])
	}
	if _, ok := claude["permissions"].(map[string]any); !ok {
		t.Fatalf("permissions not preserved: %#v", claude)
	}
	var vscode map[string]any
	must(t, json.Unmarshal(mustRead(t, vscodePath), &vscode))
	if vscode["editor.fontSize"].(float64) != 14 {
		t.Fatalf("editor.fontSize not preserved: %#v", vscode)
	}
	if _, ok := vscode[vscodeEnvKey()].(map[string]any); !ok {
		t.Fatalf("vscode env key missing: %#v", vscode)
	}
}

func TestBootstrapSeedsBootstrapSDKPoolFromGlobalHome(t *testing.T) {
	repo := setupGitRepo(t)
	chdir(t, repo)
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalSDK := filepath.Join(home, ".op", "sdk")
	must(t, os.MkdirAll(filepath.Join(globalSDK, "go", "0.1.0", "target"), 0o755))
	must(t, os.WriteFile(filepath.Join(globalSDK, "go", "0.1.0", "target", "manifest.json"), []byte("{}"), 0o644))
	must(t, os.MkdirAll(filepath.Join(globalSDK, "shared", "protoc"), 0o755))
	must(t, os.WriteFile(filepath.Join(globalSDK, "shared", "protoc", "manifest.json"), []byte("{}"), 0o644))
	restore, _ := fakeBuildCommand(t)
	defer restore()

	result, err := Create("feature/sdk-seed", ModeIsolated)
	if err != nil {
		t.Fatal(err)
	}

	if !fileExists(filepath.Join(result.Worktree, ".op", "sdk", "go", "0.1.0", "target", "manifest.json")) {
		t.Fatal("go SDK seed was not copied into isolated OPPATH")
	}
	if !fileExists(filepath.Join(result.Worktree, ".op", "sdk", "shared", "protoc", "manifest.json")) {
		t.Fatal("shared SDK seed was not copied into isolated OPPATH")
	}
}

func setupGitRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "seed")
	must(t, os.MkdirAll(root, 0o755))
	must(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\n"), 0o644))
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")
	return root
}

func fakeBuildCommand(t *testing.T) (func(), *[]string) {
	t.Helper()
	var calls []string
	restore := SetBuildCommandForTest(func(opCommand string, _ string, env []string, target string) error {
		envMap := envSliceMap(env)
		opbin := envMap["OPBIN"]
		oppath := envMap["OPPATH"]
		if opbin == "" || oppath == "" {
			t.Fatalf("build env missing OPBIN/OPPATH: %#v", envMap)
		}
		must(t, os.MkdirAll(opbin, 0o755))
		switch target {
		case "op":
			if opCommand != "op" {
				t.Fatalf("bootstrap op command = %q, want op", opCommand)
			}
			must(t, os.WriteFile(filepath.Join(opbin, "op"), []byte("worktree op"), 0o755))
		case "clem-ader":
			if opCommand != filepath.Join(opbin, "op") {
				t.Fatalf("ader build op command = %q, want local op", opCommand)
			}
			must(t, os.WriteFile(filepath.Join(opbin, "ader"), []byte("worktree ader"), 0o755))
		default:
			t.Fatalf("unexpected build target %q", target)
		}
		must(t, os.WriteFile(filepath.Join(oppath, "build.log"), []byte(target+"\n"), 0o644))
		calls = append(calls, target)
		return nil
	})
	return restore, &calls
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	must(t, os.Chdir(dir))
}

func envSliceMap(env []string) map[string]string {
	out := map[string]string{}
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			out[k] = v
		}
	}
	return out
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLookPathInEnvUsesInjectedPath(t *testing.T) {
	dir := t.TempDir()
	name := "op"
	if runtime.GOOS == "windows" {
		name = "op.exe"
	}
	bin := filepath.Join(dir, name)
	must(t, os.WriteFile(bin, []byte("fake"), 0o755))
	got, err := LookPathInEnv("op", map[string]string{"PATH": dir})
	if err != nil {
		t.Fatal(err)
	}
	if got != bin {
		t.Fatalf("LookPathInEnv = %s, want %s", got, bin)
	}
}

func TestEnsureDirectBinaryLinksInstalledHolonPackage(t *testing.T) {
	opbin := t.TempDir()
	pkgBin := filepath.Join(opbin, "clem-ader.holon", "bin", runtimeArchitecture())
	must(t, os.MkdirAll(pkgBin, 0o755))
	must(t, os.WriteFile(filepath.Join(pkgBin, "ader"), []byte("ader"), 0o755))

	if err := ensureDirectBinary(opbin, "ader"); err != nil {
		t.Fatal(err)
	}

	target, err := os.Readlink(filepath.Join(opbin, "ader"))
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join("clem-ader.holon", "bin", runtimeArchitecture(), "ader") {
		t.Fatalf("ader symlink = %q", target)
	}
}

func TestEndUserDocsAndAgentEntryPointsMentionCanonicalCommand(t *testing.T) {
	root := seedRootFromPackage(t)
	docs := string(mustRead(t, filepath.Join(root, "holons", "grace-op", "OP_WORKTREE.md")))
	for _, needle := range []string{
		"--isolated",
		"--plain",
		"op worktree launch feature/X -- codex",
		"/op-worktree feature/X isolated",
		"doctor",
		"Idempotence",
		"Promoting A Plain Worktree",
		"~/.op",
	} {
		if !strings.Contains(docs, needle) {
			t.Fatalf("OP_WORKTREE.md missing %q", needle)
		}
	}
	codexSkill := string(mustRead(t, filepath.Join(root, "plugins", "op-worktree", "skills", "op-worktree", "SKILL.md")))
	if !strings.Contains(codexSkill, "op worktree create <branch> --<mode> --json") {
		t.Fatal("Codex skill missing canonical op worktree invocation")
	}
	claudeSkill := string(mustRead(t, filepath.Join(root, ".claude", "skills", "op-worktree", "SKILL.md")))
	if !strings.Contains(claudeSkill, "op worktree create <branch> --<mode> --json") {
		t.Fatal("Claude skill missing canonical op worktree invocation")
	}
	hook := string(mustRead(t, filepath.Join(root, "plugins", "op-worktree", "hooks.json")))
	if strings.Contains(hook, "python") {
		t.Fatal("worktree hook should not invoke python")
	}
	wrapper := string(mustRead(t, filepath.Join(root, "scripts", "op-worktree")))
	if !strings.Contains(wrapper, `exec op worktree "$@"`) {
		t.Fatal("scripts/op-worktree should remain a thin op worktree wrapper")
	}
}

func seedRootFromPackage(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
}
