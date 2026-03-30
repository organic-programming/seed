// Package protosfs embeds the canonical holon proto schemas.
// These protos are immutable for a given version of op.
package protosfs

import "embed"

// FS exposes the canonical holons/v1/ proto files.
//
//go:embed holons/**
var FS embed.FS
