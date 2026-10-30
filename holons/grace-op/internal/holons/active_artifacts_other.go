//go:build !windows && !darwin

package holons

import (
	"context"
	"fmt"
)

// Other platforms retain runner-specific checks until they have an OS-backed
// executable identity provider. In particular, no process stopping is offered.
const supportsActiveArtifacts = false

func describeActiveArtifactProcess(p ActiveArtifactProcess) ActiveArtifactProcess { return p }

func activeArtifactProcesses() ([]ActiveArtifactProcess, error) { return nil, nil }

func stopActiveArtifactProcess(context.Context, ActiveArtifactProcess, bool) error {
	return fmt.Errorf("stopping active artifacts is not supported on this platform")
}
