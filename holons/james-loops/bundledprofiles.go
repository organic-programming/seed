package jamesloops

import "embed"

// BundledProfilesFS exposes the bundled profile YAML files committed with the holon.
//
//go:embed .op/profiles/*.yaml
var BundledProfilesFS embed.FS
