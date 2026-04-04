package composite_rebuild_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestBuild_08_CompositeRebuildsTouchedMember(t *testing.T) {
	rootPath := integration.DefaultWorkspaceDir(t)
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)

	childHolon := "gabriel-greeting-go"
	childBinary := buildArtifactPath(rootPath, childHolon)
	childRoot := filepath.Join(rootPath, "examples", "hello-world", childHolon)
	rebuildTrigger := filepath.Join(childRoot, "integration-rebuild-trigger.txt")

	buildChild := exec.Command(opBin, "build", childHolon, "--root", rootPath)
	buildChild.Env = envVars
	if out, err := buildChild.CombinedOutput(); err != nil {
		t.Fatalf("Failed to prebuild child holon: %v\nOutput: %s", err, string(out))
	}

	beforeInfo, err := os.Stat(childBinary)
	if err != nil {
		t.Fatalf("stat child binary before composite build: %v", err)
	}

	if err := os.WriteFile(rebuildTrigger, []byte("trigger composite member rebuild\n"), 0o644); err != nil {
		t.Fatalf("write rebuild trigger: %v", err)
	}
	defer func() {
		_ = os.Remove(rebuildTrigger)
	}()

	cmd := exec.Command(opBin, "build", "gabriel-greeting-app-swiftui", "--root", rootPath)
	cmd.Env = envVars
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to rebuild composite after touching member source: %v\nOutput: %s", err, string(out))
	}

	afterInfo, err := os.Stat(childBinary)
	if err != nil {
		t.Fatalf("stat child binary after composite build: %v", err)
	}
	if !afterInfo.ModTime().After(beforeInfo.ModTime()) {
		t.Fatalf("expected composite build to rebuild %s after source change; binary mtime stayed %s\nOutput: %s", childHolon, afterInfo.ModTime(), string(out))
	}
	if _, err := os.Stat(appBundlePath(rootPath, "gabriel-greeting-app-swiftui")); err != nil {
		t.Fatalf("expected rebuilt composite app bundle: %v", err)
	}
}

func buildArtifactPath(rootPath string, holon string) string {
	return filepath.Join(
		rootPath,
		"examples",
		"hello-world",
		holon,
		".op",
		"build",
		holon+".holon",
		"bin",
		runtime.GOOS+"_"+runtime.GOARCH,
		holon,
	)
}

func appBundlePath(rootPath string, app string) string {
	return filepath.Join(rootPath, "examples", "hello-world", app, ".op", "build", "GabrielGreetingApp.app")
}
