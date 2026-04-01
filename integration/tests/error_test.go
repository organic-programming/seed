// Error tests verify that invalid targets, payloads, and conflicting flags fail
// cleanly when routed through the real op binary.
package integration

import "testing"

func TestErrors_RobustFailures(t *testing.T) {
	sb := newSandbox(t)

	holonNotFound := sb.runOP(t, "build", "nonexistent-holon")
	requireFailure(t, holonNotFound)
	requireContains(t, holonNotFound.Stderr, "not found")

	invalidJSON := sb.runOP(t, "gabriel-greeting-go", "SayHello", "{broken")
	requireFailure(t, invalidJSON)
	requireContains(t, invalidJSON.Stderr, "invalid")

	conflictingRunFlags := sb.runOP(t, "run", "--clean", "--no-build", "gabriel-greeting-go")
	requireFailure(t, conflictingRunFlags)
	requireContains(t, conflictingRunFlags.Stderr, "--clean cannot be combined with --no-build")

	conflictingBuildFlags := sb.runOP(t, "build", "--dry-run", "--clean", "gabriel-greeting-go")
	requireFailure(t, conflictingBuildFlags)
	requireContains(t, conflictingBuildFlags.Stderr, "--clean cannot be combined with --dry-run")

	missingMethod := sb.runOP(t, "gabriel-greeting-go")
	requireFailure(t, missingMethod)
	requireContains(t, missingMethod.Stderr, "missing command")
}
