package build_test

import "testing"

func TestBuildAPI_ContractScenarios(t *testing.T) {
	for _, scenario := range buildScenarios() {
		t.Run(scenario.Name, func(t *testing.T) {
			outcome, env := runAPIScenario(t, scenario)
			scenario.assertAPIContract(t, outcome, env)
		})
	}
}
