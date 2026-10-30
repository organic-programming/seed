//go:build !windows && !darwin

package holons

import "testing"

func TestActiveArtifactGuardDoesNothingOnOtherPlatforms(t *testing.T) {
	// No manifest traversal, filesystem inspection, or process enumeration is
	// permitted on platforms that already have their own lifecycle semantics.
	for _, op := range []Operation{OperationClean, OperationBuild, OperationCheck, OperationTest} {
		if err := CheckActiveArtifacts(nil, BuildContext{}, op); err != nil {
			t.Fatalf("%s: %v", op, err)
		}
	}
}
