//go:build e2e

package inspect_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInspect_CLI_LocalHolons(t *testing.T) {
	for _, spec := range integration.NativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := integration.NewSandbox(t)

			textResult := sb.RunOP(t, "inspect", spec.Slug)
			integration.RequireSuccess(t, textResult)
			integration.RequireContains(t, textResult.Stdout, integration.ExpectedServiceName(spec))

			jsonResult := sb.RunOP(t, "inspect", spec.Slug, "--json")
			integration.RequireSuccess(t, jsonResult)

			var payload struct {
				Services []struct {
					Name string `json:"name"`
				} `json:"services"`
			}
			payload = integration.DecodeJSON[struct {
				Services []struct {
					Name string `json:"name"`
				} `json:"services"`
			}](t, jsonResult.Stdout)
			if len(payload.Services) == 0 {
				t.Fatalf("inspect %s returned no services", spec.Slug)
			}
		})
	}
}

func TestInspect_CLI_ShowsSkillsAndSequences(t *testing.T) {
	sb := integration.NewSandbox(t)
	result := sb.RunOP(t, "inspect", "gabriel-greeting-go", "--json")
	integration.RequireSuccess(t, result)

	var payload struct {
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
		Sequences []struct {
			Name string `json:"name"`
		} `json:"sequences"`
	}
	payload = integration.DecodeJSON[struct {
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
		Sequences []struct {
			Name string `json:"name"`
		} `json:"sequences"`
	}](t, result.Stdout)

	if len(payload.Skills) == 0 || payload.Skills[0].Name == "" {
		t.Fatalf("inspect skills = %#v, want at least one skill", payload.Skills)
	}
	if len(payload.Sequences) == 0 {
		t.Fatalf("inspect sequences = %#v, want at least one sequence", payload.Sequences)
	}
}

func TestInspect_CLI_RemoteDescribe(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	sb := integration.NewSandbox(t)
	process := sb.StartProcess(t, integration.RunOptions{}, "run", "gabriel-greeting-go", "--listen", "tcp://127.0.0.1:0")
	defer process.Stop(t)

	address := process.WaitForListenAddress(t, integration.ProcessStartTimeout)
	result := sb.RunOP(t, "inspect", strings.TrimPrefix(address, "tcp://"))
	integration.RequireSuccess(t, result)
	integration.RequireContains(t, result.Stdout, "GreetingService")
}
