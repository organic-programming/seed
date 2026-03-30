package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/manifestproto"
)

func WriteIdentityFile(id identity.Identity, path string) error {
	if err := EnsureManifestSupport(filepath.Dir(path)); err != nil {
		return err
	}
	return identity.WriteHolonProto(id, path)
}

func WriteManifestFile(path, body string) error {
	if err := EnsureManifestSupport(filepath.Dir(path)); err != nil {
		return err
	}

	data, err := manifestproto.RenderFromYAML([]byte(body), manifestproto.RenderOptions{
		FallbackName: filepath.Base(filepath.Dir(path)),
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest %s: %w", path, err)
	}
	return nil
}

func EnsureManifestSupport(dir string) error {
	source := filepath.Join(repoRoot(), "_protos", "holons", "v1", "manifest.proto")
	if _, err := os.Stat(source); err != nil {
		source = filepath.Join(repoRoot(), "holons", "grace-op", "_protos", "holons", "v1", "manifest.proto")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}

	targetDir := filepath.Join(dir, "holons", "v1")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", targetDir, err)
	}
	target := filepath.Join(targetDir, "manifest.proto")
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
