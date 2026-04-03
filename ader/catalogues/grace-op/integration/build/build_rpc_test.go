package build_test

import "testing"

func TestBuildRPC_SymmetricScenarios(t *testing.T) {
	for _, scenario := range buildScenarios() {
		t.Run(scenario.Name, func(t *testing.T) {
			apiOutcome, apiEnv := runAPIScenario(t, scenario)
			scenario.assertAPIContract(t, apiOutcome, apiEnv)

			cliOutcome, cliEnv := runCLIScenario(t, scenario)
			rpcOutcome, rpcEnv := runRPCScenario(t, scenario)

			scenario.assertSymmetry(t, apiOutcome, apiEnv, cliOutcome, cliEnv, rpcOutcome, rpcEnv)
		})
	}
}
