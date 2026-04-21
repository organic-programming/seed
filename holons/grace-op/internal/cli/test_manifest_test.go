package cli

import (
	"path/filepath"
	"testing"

	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/testutil"
)

func writeCLIManifest(t *testing.T, dir, body string) {
	t.Helper()
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), body); err != nil {
		t.Fatal(err)
	}
}

func writeCLIIdentity(t *testing.T, dir string, id identity.Identity) {
	t.Helper()
	if err := writeCLIIdentityFile(id, filepath.Join(dir, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}
}

func writeCLIManifestFile(path, body string) error {
	return testutil.WriteManifestFile(path, body)
}

func writeCLIIdentityFile(id identity.Identity, path string) error {
	return testutil.WriteIdentityFile(id, path)
}
