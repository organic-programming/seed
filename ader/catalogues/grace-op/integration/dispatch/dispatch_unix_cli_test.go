//go:build e2e && !windows

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
			example := integration.PrimaryInvokeExample(spec)

			process := sb.StartProcess(t, integration.RunOptions{}, "run", spec.Slug, "--listen", "unix://"+socketPath)
			defer process.Stop(t)
			process.WaitForListenAddress(t, integration.ProcessStartTimeout)

			result := sb.RunOP(t, "unix://"+socketPath, example.Method, example.Payload)
			integration.RequireSuccess(t, result)
			payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
			integration.AssertInvokePayload(t, spec, example.Method, payload)
		})
	}
}
