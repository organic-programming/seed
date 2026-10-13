//go:build !windows

package holons

import "testing"

func TestActiveWindowsArtifactGuardDoesNothingOnOtherPlatforms(t *testing.T) {
	// No manifest traversal, filesystem inspection, or process enumeration is
	// permitted on platforms that already have their own lifecycle semantics.
	for _, op := range []Operation{OperationClean, OperationBuild, OperationCheck, OperationTest} {
		if err := guardActiveWindowsArtifacts(nil, BuildContext{}, op); err != nil {
			t.Fatalf("%s: %v", op, err)
		}
	}
}
