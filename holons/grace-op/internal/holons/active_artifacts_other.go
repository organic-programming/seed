//go:build !windows

package holons

// Windows prevents removal of loaded executable images. Other platforms retain
// their existing lifecycle behavior, including any runner-specific checks.
func guardActiveWindowsArtifacts(_ *LoadedManifest, _ BuildContext, _ Operation) error {
	return nil
}
