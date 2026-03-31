package integration

import (
	"os"
	"os/exec"
	"testing"
)

func TestInstalledBinary_Smoke(t *testing.T) {
	if os.Getenv("OP_TEST_INSTALLED") != "1" {
		t.Skip("set OP_TEST_INSTALLED=1 to run installed-binary smoke tests")
	}

	installedBinary, err := exec.LookPath("op")
	if err != nil {
		t.Skip("op not found on PATH")
	}

	sb := newSandbox(t)

	versionResult := sb.runOPWithOptions(t, runOptions{BinaryPath: installedBinary}, "version")
	requireSuccess(t, versionResult)
	requireContains(t, versionResult.Stdout, "op ")

	discoverResult := sb.runOPWithOptions(t, runOptions{BinaryPath: installedBinary}, "discover")
	requireSuccess(t, discoverResult)
	requireContains(t, discoverResult.Stdout, "gabriel-greeting-go")

	buildResult := sb.runOPWithOptions(t, runOptions{BinaryPath: installedBinary}, "build", "gabriel-greeting-go")
	requireSuccess(t, buildResult)

	dispatchResult := sb.runOPWithOptions(t, runOptions{BinaryPath: installedBinary}, "gabriel-greeting-go", "SayHello", `{"name":"World","lang_code":"en"}`)
	requireSuccess(t, dispatchResult)
}
