package api

// VersionString returns the holon version.
// The template expression is resolved at build time by op build.
func VersionString() string { return "{{ .Version }}" }

func Banner() string {
	return "op " + VersionString()
}
