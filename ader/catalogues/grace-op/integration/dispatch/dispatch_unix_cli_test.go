//go:build !windows

package dispatch_test

import (
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestDispatch_CLI_UnixTransport_RunAndDispatch(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			socketPath := integration.ShortSocketPath(t, spec.Slug)

			process := sb.StartProcess(t, integration.RunOptions{}, "run", spec.Slug, "--listen", "unix://"+socketPath)
			defer process.Stop(t)
			process.WaitForListenAddress(t, integration.ProcessStartTimeout)

			result := sb.RunOP(t, "unix://"+socketPath, "SayHello", `{"name":"World","lang_code":"en"}`)
			integration.RequireSuccess(t, result)
			payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
			if payload["greeting"] == "" {
				t.Fatalf("empty unix payload: %#v", payload)
			}
		})
	}
}
