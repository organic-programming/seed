//go:build !windows

// Package processowner owns the lifetime of descendants created by a holon.
package processowner

// Ensure leaves Unix process-group and signal behavior unchanged.
func Ensure() error { return nil }
