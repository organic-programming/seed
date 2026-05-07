package api

// VersionString is replaced by op build from the holon manifest version field.
func VersionString() string { return "{{ .Version }}" }
