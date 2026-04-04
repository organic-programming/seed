package api

// VersionString returns the holon version.
// The template expression is resolved at build time by op build.
func VersionString() string { return "0.5.659" }

func Banner() string {
	return "op " + VersionString()
}
