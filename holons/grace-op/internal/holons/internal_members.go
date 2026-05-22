package holons

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/organic-programming/grace-op/internal/identity"
)

func isInternalHolonsDir(root, path, name string) bool {
	if name != "holons" || filepath.Clean(path) == filepath.Clean(root) {
		return false
	}
	return hasRootHolonManifest(filepath.Dir(path))
}

func isInsideInternalHolonsDir(root, path string) bool {
	cleanRoot := filepath.Clean(root)
	current := filepath.Clean(path)
	for {
		if current == cleanRoot {
			return false
		}
		if filepath.Base(current) == "holons" && hasRootHolonManifest(filepath.Dir(current)) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		rel, err := filepath.Rel(cleanRoot, parent)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return false
		}
		current = parent
	}
}

func hasRootHolonManifest(dir string) bool {
	for _, rel := range []string{
		identity.ProtoManifestFileName,
		filepath.Join("api", "v1", identity.ProtoManifestFileName),
		filepath.Join("v1", identity.ProtoManifestFileName),
	} {
		info, err := os.Stat(filepath.Join(dir, rel))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
