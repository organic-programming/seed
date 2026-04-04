package composite_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestBuild_07_Composite(t *testing.T) {
	rootPath := integration.DefaultWorkspaceDir(t)
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)

	t.Log("Building composite app gabriel-greeting-app-swiftui from a clean state...")
	cmd := exec.Command(opBin, "build", "gabriel-greeting-app-swiftui", "--root", rootPath)
	cmd.Env = envVars
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build composite SwiftUI app: %v\nOutput: %s", err, string(out))
	}

	if _, err := os.Stat(appBundlePath(rootPath, "gabriel-greeting-app-swiftui")); err != nil {
		t.Fatalf("expected built app bundle: %v", err)
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
