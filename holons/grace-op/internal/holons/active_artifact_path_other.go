//go:build !windows && !darwin

package holons

func canonicalExistingArtifactPath(path string) (string, error) { return path, nil }
