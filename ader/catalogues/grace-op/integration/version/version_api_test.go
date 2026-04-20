//go:build e2e

package version_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

func TestVersion_API_ReturnsBanner(t *testing.T) {
	resp, err := api.Version(&opv1.VersionRequest{})
	if err != nil {
		t.Fatalf("api.Version: %v", err)
	}
	if resp.GetName() != "op" || resp.GetBanner() == "" {
		t.Fatalf("unexpected version response: %#v", resp)
	}
}
