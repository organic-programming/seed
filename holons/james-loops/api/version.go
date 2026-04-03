package api

import "github.com/organic-programming/james-loops/gen"

// VersionString is derived from the manifest carried by the static Describe payload.
func VersionString() string {
	return gen.StaticDescribeResponse().GetManifest().GetIdentity().GetVersion()
}
