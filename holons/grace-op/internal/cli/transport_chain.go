package cli

import "fmt"

// selectTransport determines the best transport for a target holon.
// Priority:
//  1. Already running (known endpoint) -> dial existing
//  2. Binary available locally -> stdio:// (ephemeral)
//  3. Network reachable -> tcp://
func selectTransport(holonName string) (scheme string, err error) {
	binaryPath, err := resolveHolon(holonName)
	if binaryPath != "" {
		return "stdio", nil
	}

	return "", fmt.Errorf("holon not reachable")
}
