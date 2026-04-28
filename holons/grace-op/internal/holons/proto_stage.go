package holons

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	protosfs "github.com/organic-programming/grace-op/_protos"
	"github.com/organic-programming/grace-op/internal/progress"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

// protoStage runs the proto-first pre-build phase:
//  1. Wipe .op/protos/ and .op/pb/
//  2. Copy embedded canonical protos to .op/protos/
//  3. Copy the holon's own proto files to .op/protos/
//  4. Parse all staged protos via protocompile
//  5. Write a FileDescriptorSet to .op/pb/descriptors.binpb
//
// Parse failure stops the build — proto breakage is caught here.
func protoStage(manifest *LoadedManifest, reporter progress.Reporter) error {
	opRoot := manifest.OpRoot()
	protosDir := filepath.Join(opRoot, "protos")
	pbDir := filepath.Join(opRoot, "pb")

	reporter.Step("proto stage: staging protos...")

	// Wipe previous staging.
	if err := os.RemoveAll(protosDir); err != nil {
		return fmt.Errorf("proto stage: clean protos: %w", err)
	}
	if err := os.RemoveAll(pbDir); err != nil {
		return fmt.Errorf("proto stage: clean pb: %w", err)
	}
	if err := os.MkdirAll(protosDir, 0755); err != nil {
		return fmt.Errorf("proto stage: create protos dir: %w", err)
	}
	if err := os.MkdirAll(pbDir, 0755); err != nil {
		return fmt.Errorf("proto stage: create pb dir: %w", err)
	}

	// Stage the holon's own proto files first (they take precedence).
	holopProtos, err := stageHolonProtos(manifest, protosDir)
	if err != nil {
		return fmt.Errorf("proto stage: holon protos: %w", err)
	}

	// No holon protos → skip the rest (legacy YAML-based holons).
	if len(holopProtos) == 0 {
		return nil
	}

	// Stage shared _protos/ directories from ancestor paths.
	// This handles shared contract protos (e.g., examples/_protos/v1/greeting.proto).
	if err := stageAncestorProtos(manifest.Dir, protosDir); err != nil {
		return fmt.Errorf("proto stage: ancestor protos: %w", err)
	}

	// Stage embedded canonical protos, skipping any already provided by the holon.
	if err := stageEmbeddedProtos(protosDir); err != nil {
		return fmt.Errorf("proto stage: embed: %w", err)
	}

	reporter.Step("proto stage: parsing...")

	// Parse all staged protos.
	fds, err := parseStaged(protosDir, holopProtos)
	if err != nil {
		return fmt.Errorf("proto stage: %w", err)
	}

	// Write the FileDescriptorSet.
	descriptorPath := filepath.Join(pbDir, "descriptors.binpb")
	if err := writeDescriptorSet(fds, descriptorPath); err != nil {
		return fmt.Errorf("proto stage: %w", err)
	}

	// Generate reference documentation from proto comments.
	if err := generateDocumentation(manifest, fds, reporter); err != nil {
		return fmt.Errorf("proto stage: %w", err)
	}

	reporter.Step(fmt.Sprintf("proto stage: %s", workspaceRelativePath(descriptorPath)))
	return nil
}

// stageEmbeddedProtos copies all .proto files from the embedded FS to the staging directory.
func stageEmbeddedProtos(destDir string) error {
	return fs.WalkDir(protosfs.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(destDir, filepath.FromSlash(path)), 0755)
		}
		if !strings.HasSuffix(path, ".proto") {
			return nil
		}

		data, readErr := fs.ReadFile(protosfs.FS, path)
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", path, readErr)
		}
		destPath := filepath.Join(destDir, filepath.FromSlash(path))
		// Skip if the holon already staged this file.
		if _, err := os.Stat(destPath); err == nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	})
}

// stageAncestorProtos walks up from holonDir, finds _protos/ directories,
// and copies their .proto files into the staging area. This enables shared
// contract protos (e.g., examples/_protos/v1/greeting.proto) to be available
// during the proto stage parse. Files already staged are skipped.
func stageAncestorProtos(holonDir, destDir string) error {
	for current := filepath.Dir(holonDir); current != "" && current != filepath.Dir(current); current = filepath.Dir(current) {
		protosPath := filepath.Join(current, "_protos")
		info, err := os.Stat(protosPath)
		if err != nil || !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(protosPath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".proto") {
				return nil
			}
			relPath, err := filepath.Rel(protosPath, path)
			if err != nil {
				return err
			}
			destPath := filepath.Join(destDir, relPath)
			// Skip if already staged by the holon or a closer ancestor.
			if _, err := os.Stat(destPath); err == nil {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read %s: %w", relPath, readErr)
			}
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			return os.WriteFile(destPath, data, 0644)
		})
		if err != nil {
			return fmt.Errorf("stage %s: %w", protosPath, err)
		}
	}
	return nil
}

// stageHolonProtos copies the holon's own .proto files into the staging directory,
// preserving their relative structure. Returns the staged relative paths.
func stageHolonProtos(manifest *LoadedManifest, destDir string) ([]string, error) {
	holonDir := manifest.Dir
	var staged []string

	err := filepath.WalkDir(holonDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			base := d.Name()
			// Skip build output, hidden dirs, vendor, node_modules, Python venvs.
			if base == ".op" || base == ".git" || base == ".build" || base == "build" ||
				base == "vendor" || base == "node_modules" || base == "gen" ||
				base == ".venv" || base == "__pycache__" || base == ".bundle" {
				return filepath.SkipDir
			}
			// Skip subdirectories that are themselves holons — they get
			// their own proto stage when built as recipe members.
			if path != holonDir {
				childManifest := filepath.Join(path, "holons", "v1", "manifest.proto")
				if _, err := os.Stat(childManifest); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".proto") {
			return nil
		}

		relPath, err := filepath.Rel(holonDir, path)
		if err != nil {
			return err
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", relPath, readErr)
		}

		destPath := filepath.Join(destDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
		staged = append(staged, relPath)
		return nil
	})

	return staged, err
}

// parseStaged compiles all staged protos, returning the parsed file descriptors.
func parseStaged(protosDir string, holonProtos []string) ([]protoreflect.FileDescriptor, error) {
	if len(holonProtos) == 0 {
		return nil, nil
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{protosDir},
		}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}

	compiled, err := compiler.Compile(context.Background(), holonProtos...)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	fds := make([]protoreflect.FileDescriptor, 0, len(compiled))
	for _, fd := range compiled {
		fds = append(fds, fd)
	}
	return fds, nil
}

// writeDescriptorSet serializes the file descriptors to a FileDescriptorSet binary.
func writeDescriptorSet(fds []protoreflect.FileDescriptor, path string) error {
	fdSet := &descriptorpb.FileDescriptorSet{}

	// Collect all transitive dependencies (deduplicated).
	seen := make(map[string]bool)
	var collect func(fd protoreflect.FileDescriptor)
	collect = func(fd protoreflect.FileDescriptor) {
		if fd == nil {
			return
		}
		name := fd.Path()
		if seen[name] {
			return
		}
		seen[name] = true
		imports := fd.Imports()
		for i := 0; i < imports.Len(); i++ {
			collect(imports.Get(i).FileDescriptor)
		}
		fdSet.File = append(fdSet.File, protodesc.ToFileDescriptorProto(fd))
	}
	for _, fd := range fds {
		collect(fd)
	}

	data, err := proto.Marshal(fdSet)
	if err != nil {
		return fmt.Errorf("marshal descriptor set: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
