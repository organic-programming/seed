//go:build !windows

// Unix-only transport tests launch holons on unix sockets and dispatch real
// RPCs over that endpoint.
package integration

import (
	"path/filepath"
	"testing"
)

func TestUnixTransport_RunAndDispatch(t *testing.T) {
	skipIfShort(t, shortTestReason)

	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)
			socketPath := filepath.Join(sb.Root, "h.sock")

			process := sb.startProcess(t, runOptions{}, "run", spec.Slug, "--listen", "unix://"+socketPath)
			defer process.Stop(t)
			process.waitForListenAddress(t, processStartTimeout)

			result := sb.runOP(t, "unix://"+socketPath, "SayHello", `{"name":"World","lang_code":"en"}`)
			requireSuccess(t, result)
			payload := decodeJSON[map[string]any](t, result.Stdout)
			if payload["greeting"] == "" {
				t.Fatalf("empty unix payload: %#v", payload)
			}
		})
	}
}
