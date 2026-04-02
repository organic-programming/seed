// Run tests start real holon servers, connect to their advertised endpoints,
// and verify launch behavior across build-related flags.
package integration

import (
	"fmt"
	"testing"
	"time"
)

func TestRun_TCP(t *testing.T) {
	skipIfShort(t, shortTestReason)

	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)
			process := sb.startProcess(t, runOptions{}, "run", spec.Slug, "--listen", "tcp://127.0.0.1:0")
			defer process.Stop(t)

			address := process.waitForListenAddress(t, processStartTimeout)
			result := sb.runOP(t, address, "SayHello", `{"name":"World","lang_code":"en"}`)
			requireSuccess(t, result)
			payload := decodeJSON[map[string]any](t, result.Stdout)
			if payload["greeting"] == "" {
				t.Fatalf("empty run payload: %#v", payload)
			}
		})
	}
}

func TestRun_AutoBuild(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	removeArtifactFor(t, sb, "gabriel-greeting-go")

	process := sb.startProcess(t, runOptions{}, "run", "gabriel-greeting-go", "--listen", "tcp://127.0.0.1:0")
	defer process.Stop(t)

	address := process.waitForListenAddress(t, processStartTimeout)
	result := sb.runOP(t, address, "SayHello", `{"name":"World","lang_code":"en"}`)
	requireSuccess(t, result)
	requirePathExists(t, artifactPathFor(t, sb, "gabriel-greeting-go"))
}

func TestRun_NoBuildFails(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	removeArtifactFor(t, sb, "gabriel-greeting-go")

	result := sb.runOP(t, "run", "--no-build", "gabriel-greeting-go")
	requireFailure(t, result)
	requireContains(t, result.Stderr, "artifact missing")
}

func TestRun_CleanRebuildRun(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	process := sb.startProcess(t, runOptions{}, "run", "gabriel-greeting-go", "--clean", "--listen", "tcp://127.0.0.1:0")
	defer process.Stop(t)

	address := process.waitForListenAddress(t, processStartTimeout)
	result := sb.runOP(t, address, "SayHello", `{"name":"World","lang_code":"en"}`)
	requireSuccess(t, result)
}

func TestRun_PortShorthand(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	port := availablePort(t)
	process := sb.startProcess(t, runOptions{}, "run", fmt.Sprintf("gabriel-greeting-go:%d", port))
	defer process.Stop(t)

	waitUntil(t, processStartTimeout, func() bool {
		result := sb.runOPWithOptions(t, runOptions{Timeout: 2 * time.Second}, fmt.Sprintf("tcp://127.0.0.1:%d", port), "SayHello", `{"name":"World","lang_code":"en"}`)
		return result.Err == nil
	})
}
