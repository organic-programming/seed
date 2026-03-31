package integration

import (
	"strings"
	"testing"
)

func TestInspect_LocalHolons(t *testing.T) {
	for _, spec := range nativeTestHolons(t) {
		t.Run(spec.Slug, func(t *testing.T) {
			sb := newSandbox(t)

			textResult := sb.runOP(t, "inspect", spec.Slug)
			requireSuccess(t, textResult)
			requireContains(t, textResult.Stdout, "GreetingService")

			jsonResult := sb.runOP(t, "inspect", spec.Slug, "--json")
			requireSuccess(t, jsonResult)
			var payload struct {
				Services []struct {
					Name string `json:"name"`
				} `json:"services"`
			}
			payload = decodeJSON[struct {
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

func TestInspect_ShowsSkillsAndSequences(t *testing.T) {
	sb := newSandbox(t)
	result := sb.runOP(t, "inspect", "gabriel-greeting-go", "--json")
	requireSuccess(t, result)

	var payload struct {
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
		Sequences []struct {
			Name string `json:"name"`
		} `json:"sequences"`
	}
	payload = decodeJSON[struct {
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

func TestInspect_RemoteDescribe(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	process := sb.startProcess(t, runOptions{}, "run", "gabriel-greeting-go", "--listen", "tcp://127.0.0.1:0")
	defer process.Stop(t)

	address := process.waitForListenAddress(t, processStartTimeout)
	result := sb.runOP(t, "inspect", strings.TrimPrefix(address, "tcp://"))
	requireSuccess(t, result)
	requireContains(t, result.Stdout, "GreetingService")
}
