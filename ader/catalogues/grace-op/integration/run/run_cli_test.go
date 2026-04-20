//go:build e2e

package run_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestRun_CLI_TCP(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)
			example := integration.PrimaryInvokeExample(spec)
			process := sb.StartProcess(t, integration.RunOptions{}, "run", spec.Slug, "--listen", "tcp://127.0.0.1:0")
			defer process.Stop(t)

			address := process.WaitForListenAddress(t, integration.ProcessStartTimeout)
			result := sb.RunOP(t, address, example.Method, example.Payload)
			integration.RequireSuccess(t, result)
			payload := integration.DecodeJSON[map[string]any](t, result.Stdout)
			integration.AssertInvokePayload(t, spec, example.Method, payload)
		})
	}
}

func TestRun_CLI_AutoBuild(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	sb := integration.NewSandbox(t)
	integration.RemoveArtifactFor(t, sb, "gabriel-greeting-go")
	example := integration.PrimaryInvokeExample(integration.HolonSpec{Slug: "gabriel-greeting-go"})

	process := sb.StartProcess(t, integration.RunOptions{}, "run", "gabriel-greeting-go", "--listen", "tcp://127.0.0.1:0")
	defer process.Stop(t)

	address := process.WaitForListenAddress(t, integration.ProcessStartTimeout)
	result := sb.RunOP(t, address, example.Method, example.Payload)
	integration.RequireSuccess(t, result)
	integration.RequirePathExists(t, integration.ArtifactPathFor(t, sb, "gabriel-greeting-go"))
}

func TestRun_CLI_NoBuildFails(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	sb := integration.NewSandbox(t)
	integration.RemoveArtifactFor(t, sb, "gabriel-greeting-go")

	result := sb.RunOP(t, "run", "--no-build", "gabriel-greeting-go")
	integration.RequireFailure(t, result)
	integration.RequireContains(t, result.Stderr, "artifact missing")
}

func TestRun_CLI_CleanRebuildRun(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	sb := integration.NewSandbox(t)
	process := sb.StartProcess(t, integration.RunOptions{}, "run", "gabriel-greeting-go", "--clean", "--listen", "tcp://127.0.0.1:0")
	defer process.Stop(t)

	address := process.WaitForListenAddress(t, integration.ProcessStartTimeout)
	example := integration.PrimaryInvokeExample(integration.HolonSpec{Slug: "gabriel-greeting-go"})
	result := sb.RunOP(t, address, example.Method, example.Payload)
	integration.RequireSuccess(t, result)
}

func TestRun_CLI_PortShorthand(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	sb := integration.NewSandbox(t)
	port := integration.AvailablePort(t)
	process := sb.StartProcess(t, integration.RunOptions{}, "run", fmt.Sprintf("gabriel-greeting-go:%d", port))
	defer process.Stop(t)

	example := integration.PrimaryInvokeExample(integration.HolonSpec{Slug: "gabriel-greeting-go"})
	integration.WaitUntil(t, integration.ProcessStartTimeout, func() bool {
		result := sb.RunOPWithOptions(t, integration.RunOptions{Timeout: 2 * time.Second}, fmt.Sprintf("tcp://127.0.0.1:%d", port), example.Method, example.Payload)
		return result.Err == nil
	})
}
