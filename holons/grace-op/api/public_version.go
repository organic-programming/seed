package api

import (
	"fmt"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

func Version(_ *opv1.VersionRequest) (*opv1.VersionResponse, error) {
	return VersionWithString(VersionString()), nil
}

func VersionWithString(version string) *opv1.VersionResponse {
	trimmed := strings.TrimSpace(version)
	name := "op"
	banner := name
	if trimmed != "" {
		banner = fmt.Sprintf("%s %s", name, trimmed)
	}

	return &opv1.VersionResponse{
		Name:    name,
		Version: trimmed,
		Banner:  banner,
	}
}
