//go:build !windows

package holons

func bundleCargoWindowsCRT(*LoadedManifest, *Report) error { return nil }
