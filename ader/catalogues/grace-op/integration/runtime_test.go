package integration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareWorkspaceMirrorRemovesStaleFiles(t *testing.T) {
	seedRoot := t.TempDir()
	artifactsRoot := t.TempDir()

	sourceFile := filepath.Join(seedRoot, "examples", "hello.txt")
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("fresh"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	scriptModuleFile := filepath.Join(seedRoot, ".github", "scripts", "go.mod")
	if err := os.MkdirAll(filepath.Dir(scriptModuleFile), 0o755); err != nil {
		t.Fatalf("mkdir script module: %v", err)
	}
	if err := os.WriteFile(scriptModuleFile, []byte("module scripts\n"), 0o644); err != nil {
		t.Fatalf("write script module: %v", err)
	}

	staleFile := filepath.Join(artifactsRoot, "workspace", "examples", "stale.txt")
	if err := os.MkdirAll(filepath.Dir(staleFile), 0o755); err != nil {
		t.Fatalf("mkdir stale: %v", err)
	}
	if err := os.WriteFile(staleFile, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}

	rt := &runtimeState{
		seedRoot:      seedRoot,
		artifactsRoot: artifactsRoot,
	}
	mirrorRoot, err := prepareWorkspaceMirror(rt)
	if err != nil {
		t.Fatalf("prepareWorkspaceMirror: %v", err)
	}

	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Fatalf("stale file still exists after mirror preparation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mirrorRoot, "examples", "hello.txt")); err != nil {
		t.Fatalf("fresh source file was not mirrored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mirrorRoot, ".github", "scripts", "go.mod")); err != nil {
		t.Fatalf("script module was not mirrored: %v", err)
	}
}

func TestLinkExternalSDKPrebuiltsFromEnv(t *testing.T) {
	t.Setenv("OPPATH", "")

	outer := t.TempDir()
	src := filepath.Join(outer, "sdk", "go", "0.1.0", "aarch64-apple-darwin")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir source SDK: %v", err)
	}
	t.Setenv("OP_SDK_GO_PATH", src)

	sandboxOPPATH := filepath.Join(t.TempDir(), ".op")
	linkExternalSDKPrebuilts(t, sandboxOPPATH)

	dst := filepath.Join(sandboxOPPATH, "sdk", "go", "0.1.0", "aarch64-apple-darwin")
	assertSymlinkTarget(t, dst, src)
}

func TestLinkExternalSDKPrebuiltsFromOuterOPPATH(t *testing.T) {
	outerOPPATH := filepath.Join(t.TempDir(), "op")
	src := filepath.Join(outerOPPATH, "sdk", "dart", "0.1.0", "aarch64-apple-darwin")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir source SDK: %v", err)
	}
	t.Setenv("OPPATH", outerOPPATH)

	sandboxOPPATH := filepath.Join(t.TempDir(), ".op")
	linkExternalSDKPrebuilts(t, sandboxOPPATH)

	dst := filepath.Join(sandboxOPPATH, "sdk", "dart", "0.1.0", "aarch64-apple-darwin")
	assertSymlinkTarget(t, dst, src)
}

func assertSymlinkTarget(t *testing.T, linkPath, want string) {
	t.Helper()

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat %s: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is mode %v, want symlink", linkPath, info.Mode())
	}
	got, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink %s: %v", linkPath, err)
	}
	if got != want {
		t.Fatalf("readlink %s = %s, want %s", linkPath, got, want)
	}
}
