package holons

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ActiveArtifactProcess identifies one executable image in a protected output.
// Started is an opaque OS start-time token, used with PID and canonical Path to
// avoid signalling a replacement process after PID reuse.
type ActiveArtifactProcess struct {
	PID        int    `json:"pid"`
	Path       string `json:"path"`
	Started    string `json:"started"`
	BundlePath string `json:"bundle_path,omitempty"`
}

// ActiveArtifactConflict is returned before build/clean mutates any output.
// It is deliberately independent of terminal IO and recipe command errors.
type ActiveArtifactConflict struct {
	Operation Operation               `json:"operation"`
	Holon     string                  `json:"holon"`
	Processes []ActiveArtifactProcess `json:"processes"`
}

func (e *ActiveArtifactConflict) Error() string {
	var message strings.Builder
	fmt.Fprintf(&message, "cannot %s holon %q: running processes are using its build artifacts:", e.Operation, e.Holon)
	for _, p := range e.Processes {
		fmt.Fprintf(&message, "\n  %s (PID %d): %s", filepath.Base(p.Path), p.PID, p.Path)
	}
	message.WriteString("\nClose the listed application(s) and worker(s), then retry.")
	return message.String()
}

// ErrActiveArtifactProcessChanged means that a previously inventoried PID no
// longer identifies the same executable and start time. No signal was sent.
var ErrActiveArtifactProcessChanged = errors.New("active artifact process identity changed; no stop requested")

type activeArtifactRoot struct {
	path      string
	directory bool
}

// CheckActiveArtifacts inventories the entire dependency graph before any
// member can be changed, including clean triggered by a build-state change.
// Callers must not treat prior consent as permission to bypass this check.
func CheckActiveArtifacts(manifest *LoadedManifest, ctx BuildContext, op Operation) error {
	if !supportsActiveArtifacts || (op != OperationClean && (op != OperationBuild || ctx.DryRun)) {
		return nil
	}
	roots, err := activeArtifactRoots(manifest, ctx)
	if err != nil {
		return fmt.Errorf("check running build artifacts: %w", err)
	}
	if len(roots) == 0 {
		return nil
	}
	processes, err := activeArtifactProcesses()
	if err != nil {
		return fmt.Errorf("check running build artifacts: %w", err)
	}
	return activeArtifactConflict(manifestSlug(manifest), op, roots, processes)
}

func activeArtifactConflict(holon string, op Operation, roots []activeArtifactRoot, processes []ActiveArtifactProcess) error {
	var active []ActiveArtifactProcess
	for _, p := range processes {
		canonical, err := canonicalArtifactPath(p.Path)
		if err != nil {
			// Protected executables and paths disappearing during inventory may
			// not be inspectable. Preserve the Windows snapshot semantics.
			continue
		}
		for _, root := range roots {
			if artifactContains(root, canonical) {
				p.Path = canonical
				p = describeActiveArtifactProcess(p)
				active = append(active, p)
				break
			}
		}
	}
	if len(active) == 0 {
		return nil
	}
	sort.Slice(active, func(i, j int) bool { return active[i].PID < active[j].PID })
	return &ActiveArtifactConflict{Operation: op, Holon: holon, Processes: active}
}

// RequestActiveArtifactStop only stops the exact, revalidated process supplied
// by an interactive caller. The lifecycle and API never invoke it themselves.
func RequestActiveArtifactStop(ctx context.Context, process ActiveArtifactProcess, force bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if process.PID <= 0 || process.Path == "" || process.Started == "" {
		return ErrActiveArtifactProcessChanged
	}
	return stopActiveArtifactProcess(ctx, process, force)
}

func artifactContains(root activeArtifactRoot, path string) bool {
	return root.path == path || (root.directory && strings.HasPrefix(path, strings.TrimRight(root.path, string(filepath.Separator))+string(filepath.Separator)))
}

func activeArtifactRoots(manifest *LoadedManifest, ctx BuildContext) ([]activeArtifactRoot, error) {
	manifests, err := activeArtifactManifests(manifest)
	if err != nil {
		return nil, err
	}
	var roots []activeArtifactRoot
	seen := make(map[activeArtifactRoot]bool)
	for _, current := range manifests {
		if current == nil {
			continue
		}
		paths := []string{current.OpRoot(), current.HolonPackageDir(), current.ArtifactPath(ctx)}
		// Aggregate targets have no single ArtifactPath, but can still clean or
		// replace a declared primary artifact while building individual targets.
		if primary := strings.TrimSpace(current.Manifest.Artifacts.Primary); primary != "" {
			paths = append(paths, current.mustResolveManifestPath(primary))
		}
		for _, path := range paths {
			if path == "" {
				continue
			}
			canonical, err := canonicalArtifactPath(path)
			if err != nil {
				return nil, fmt.Errorf("resolve artifact path %q: %w", path, err)
			}
			root := activeArtifactRoot{path: canonical, directory: true}
			if info, err := os.Stat(path); err == nil {
				root.directory = info.IsDir()
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("inspect artifact path %q: %w", path, err)
			}
			if !seen[root] {
				seen[root] = true
				roots = append(roots, root)
			}
		}
	}
	return roots, nil
}

// Unlike dependency builds, clean visits distinct source directories even if
// they have the same UUID. Also include hardened-excluded members: a build can
// clean those when its persisted configuration changes.
func activeArtifactManifests(manifest *LoadedManifest) ([]*LoadedManifest, error) {
	var manifests []*LoadedManifest
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var visit func(*LoadedManifest) error
	visit = func(current *LoadedManifest) error {
		if current == nil {
			return nil
		}
		key, err := canonicalArtifactPath(current.Dir)
		if err != nil {
			return err
		}
		if visiting[key] {
			return fmt.Errorf("composite dependency cycle at %q", current.Dir)
		}
		if visited[key] {
			return nil
		}
		visiting[key] = true
		manifests = append(manifests, current)
		for _, member := range current.Manifest.Build.Members {
			if member.Type != "holon" {
				continue
			}
			dir, err := current.ResolveManifestPath(member.Path)
			if err != nil {
				return fmt.Errorf("composite member %q path: %w", member.ID, err)
			}
			child, err := LoadManifest(dir)
			if err != nil {
				return err
			}
			if err := visit(child); err != nil {
				return err
			}
		}
		delete(visiting, key)
		visited[key] = true
		return nil
	}
	if err := visit(manifest); err != nil {
		return nil, err
	}
	return manifests, nil
}
