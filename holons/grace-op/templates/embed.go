package templatesfs

import "embed"

// FS embeds the shipped scaffold templates.
//
//go:embed catalog/**
var FS embed.FS
