//go:build windows

package holons

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type activeArtifactRoot struct {
	path      string
	directory bool
}

type activeArtifactProcess struct {
	pid  uint32
	path string
}

// Check the complete dependency graph before a runner can mutate any member.
// Checking only immediately before RemoveAll would leave composites partly
// cleaned when their application or one of their workers is still running.
func guardActiveWindowsArtifacts(manifest *LoadedManifest, ctx BuildContext, op Operation) error {
	if op != OperationClean && (op != OperationBuild || ctx.DryRun) {
		return nil
	}
	roots, err := activeWindowsArtifactRoots(manifest, ctx)
	if err != nil {
		return fmt.Errorf("check running build artifacts: %w", err)
	}
	processes, err := activeWindowsProcesses()
	if err != nil {
		return fmt.Errorf("check running build artifacts: %w", err)
	}
	var active []activeArtifactProcess
	for _, process := range processes {
		canonical, err := canonicalWindowsArtifactPath(process.path)
		if err != nil {
			// A process may exit while the snapshot is being examined.
			continue
		}
		for _, root := range roots {
			if windowsArtifactContains(root, canonical) {
				active = append(active, process)
				break
			}
		}
	}
	if len(active) == 0 {
		return nil
	}
	sort.Slice(active, func(i, j int) bool { return active[i].pid < active[j].pid })
	var message strings.Builder
	fmt.Fprintf(&message, "cannot %s holon %q: running processes are using its build artifacts:", op, manifestSlug(manifest))
	for _, process := range active {
		fmt.Fprintf(&message, "\n  %s (PID %d): %s", filepath.Base(process.path), process.pid, process.path)
	}
	message.WriteString("\nClose the listed application(s) and worker(s), then retry.")
	return errors.New(message.String())
}

func activeWindowsArtifactRoots(manifest *LoadedManifest, ctx BuildContext) ([]activeArtifactRoot, error) {
	manifests, err := activeWindowsArtifactManifests(manifest)
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
			canonical, err := canonicalWindowsArtifactPath(path)
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
func activeWindowsArtifactManifests(manifest *LoadedManifest) ([]*LoadedManifest, error) {
	var manifests []*LoadedManifest
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var visit func(*LoadedManifest) error
	visit = func(current *LoadedManifest) error {
		if current == nil {
			return nil
		}
		key, err := canonicalWindowsArtifactPath(current.Dir)
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

// Resolve existing ancestors as well: a clean checkout may not yet have an
// artifact directory, while its checkout path can already contain a junction.
func canonicalWindowsArtifactPath(path string) (string, error) {
	path = strings.ReplaceAll(path, "/", `\`)
	if strings.HasPrefix(path, `\\?\UNC\`) {
		path = `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	} else {
		path = strings.TrimPrefix(path, `\\?\`)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return strings.ToLower(filepath.Clean(resolved)), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}

func windowsArtifactContains(root activeArtifactRoot, path string) bool {
	if strings.EqualFold(root.path, path) {
		return true
	}
	return root.directory && strings.HasPrefix(strings.ToLower(path), strings.TrimRight(strings.ToLower(root.path), `\`)+`\`)
}

func activeWindowsProcesses() ([]activeArtifactProcess, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	err = windows.Process32First(snapshot, &entry)
	var processes []activeArtifactProcess
	for err == nil {
		// Protected/system processes cannot be queried by an ordinary user;
		// processes that exit after the snapshot also cannot be opened.
		process, openErr := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if openErr == nil {
			buffer := make([]uint16, 32768)
			size := uint32(len(buffer))
			queryErr := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size)
			windows.CloseHandle(process)
			if queryErr == nil {
				processes = append(processes, activeArtifactProcess{pid: entry.ProcessID, path: windows.UTF16ToString(buffer[:size])})
			}
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return processes, nil
}
