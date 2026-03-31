package api_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/organic-programming/go-holons/pkg/discover"
)

func TestGabrielGreetingGoIsDiscoverable(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")

	result := discover.Discover(discover.LOCAL, nil, &repoRoot, discover.SOURCE, discover.NO_LIMIT, discover.NO_TIMEOUT)
	if result.Error != "" {
		t.Fatalf("Discover error: %s", result.Error)
	}

	found := false
	for _, ref := range result.Found {
		if ref.Info != nil && ref.Info.Slug == "gabriel-greeting-go" {
			found = true
			if ref.Info.UUID != "3f08b5c3-8931-46d0-847a-a64d8b9ba57e" {
				t.Fatalf("UUID mismatch: got %q", ref.Info.UUID)
			}
			break
		}
	}
	if !found {
		t.Fatal("gabriel-greeting-go not discovered from repo root")
	}
}
