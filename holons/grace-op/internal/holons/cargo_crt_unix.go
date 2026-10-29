//go:build !windows

package holons

func bundleCargoWindowsCRT(*LoadedManifest, *Report, ...[]string) error { return nil }
