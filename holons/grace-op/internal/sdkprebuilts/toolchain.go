package sdkprebuilts

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const seedToolchainFile = "seed-toolchain.yaml"
const sharedSeedReleaseSnapshotName = "seed-release.json"

type ToolchainEntry struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Target  string `json:"target,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Source  string `json:"source,omitempty"`
}

type SeedToolchain struct {
	SeedRelease string `yaml:"seed_release"`
	Protoc      struct {
		UpstreamTag     string            `yaml:"upstream_tag"`
		Version         string            `yaml:"version"`
		RequiredBy      map[string]bool   `yaml:"required_by"`
		SHA256PerTarget map[string]string `yaml:"sha256_per_target"`
	} `yaml:"protoc"`
	CPPRuntime struct {
		ProtobufSubmoduleTag string `yaml:"protobuf_submodule_tag"`
	} `yaml:"cpp_runtime"`
	Plugins map[string]map[string]any `yaml:"plugins"`
}

type sharedToolManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Target      string `json:"target"`
	SHA256      string `json:"sha256,omitempty"`
	Source      string `json:"source,omitempty"`
	InstalledAt string `json:"installed_at"`
}

type sharedSeedReleaseSnapshot struct {
	SeedRelease string           `json:"seed_release"`
	Toolchain   []ToolchainEntry `json:"toolchain,omitempty"`
	UpdatedAt   string           `json:"updated_at"`
}

func LoadSeedToolchain(repoRoot string) (SeedToolchain, error) {
	path := filepath.Join(repoRoot, seedToolchainFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return SeedToolchain{}, err
	}
	var toolchain SeedToolchain
	if err := yaml.Unmarshal(data, &toolchain); err != nil {
		return SeedToolchain{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return toolchain, nil
}

func ToolchainForSDK(repoRoot, lang, target string) ([]ToolchainEntry, error) {
	normalized, err := NormalizeLang(lang)
	if err != nil {
		return nil, err
	}
	normalizedTarget, err := NormalizeTarget(target)
	if err != nil {
		return nil, err
	}
	seed, err := LoadSeedToolchain(repoRoot)
	if err != nil {
		return nil, err
	}

	entries := make([]ToolchainEntry, 0)
	if sdkRequiresSharedProtoc(seed, normalized) {
		version := protocVersion(seed)
		if version == "" {
			return nil, fmt.Errorf("%s missing protoc version", seedToolchainFile)
		}
		sha := strings.TrimSpace(seed.Protoc.SHA256PerTarget[normalizedTarget])
		if sha == "" {
			return nil, fmt.Errorf("%s missing protoc sha256 for %s", seedToolchainFile, normalizedTarget)
		}
		entries = append(entries, ToolchainEntry{
			Name:    "protoc",
			Version: version,
			Target:  normalizedTarget,
			SHA256:  sha,
		})
	}

	for name, raw := range seed.Plugins[normalized] {
		entry := ToolchainEntry{Name: strings.TrimSpace(name)}
		switch value := raw.(type) {
		case string:
			entry.Version = strings.TrimSpace(value)
		case map[string]any:
			if version, ok := value["version"].(string); ok {
				entry.Version = strings.TrimSpace(version)
			}
			if sha, ok := value["sha256"].(string); ok {
				entry.SHA256 = strings.TrimSpace(sha)
			}
			if perTarget, ok := value["sha256_per_target"].(map[string]any); ok {
				if sha, ok := perTarget[normalizedTarget].(string); ok {
					entry.Target = normalizedTarget
					entry.SHA256 = strings.TrimSpace(sha)
				}
			}
		}
		if entry.Name != "" {
			entries = append(entries, entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Name == "protoc" {
			return true
		}
		if entries[j].Name == "protoc" {
			return false
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func sdkRequiresSharedProtoc(seed SeedToolchain, lang string) bool {
	return seed.Protoc.RequiredBy[strings.TrimSpace(lang)]
}

func protocVersion(seed SeedToolchain) string {
	if version := strings.TrimSpace(seed.Protoc.Version); version != "" {
		return strings.TrimPrefix(version, "v")
	}
	return strings.TrimPrefix(strings.TrimSpace(seed.Protoc.UpstreamTag), "v")
}

func ensureSharedToolchain(ctx context.Context, entries []ToolchainEntry) ([]string, error) {
	var notes []string
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) != "protoc" {
			continue
		}
		note, err := ensureSharedProtoc(ctx, entry)
		if err != nil {
			return notes, err
		}
		if note != "" {
			notes = append(notes, note)
		}
	}
	return notes, nil
}

func EnsureSharedToolchain(ctx context.Context, entries []ToolchainEntry) ([]string, error) {
	return ensureSharedToolchain(ctx, entries)
}

func ensureSharedToolchainForPrebuilt(ctx context.Context, prebuilt Prebuilt) ([]string, error) {
	notes, err := ensureSharedToolchain(ctx, prebuilt.Toolchain)
	if err != nil {
		return notes, err
	}
	if err := writeSharedSeedReleaseSnapshot(prebuilt.SeedRelease, prebuilt.Toolchain); err != nil {
		return notes, err
	}
	return notes, nil
}

func writeSharedSeedReleaseSnapshot(seedRelease string, entries []ToolchainEntry) error {
	seedRelease = strings.TrimSpace(seedRelease)
	if seedRelease == "" || !containsSharedTool(entries) {
		return nil
	}
	root := filepath.Join(SDKRoot(), "shared")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", root, err)
	}
	snapshot := sharedSeedReleaseSnapshot{
		SeedRelease: seedRelease,
		Toolchain:   entries,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(root, sharedSeedReleaseSnapshotName+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(root, sharedSeedReleaseSnapshotName)); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func containsSharedTool(entries []ToolchainEntry) bool {
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "protoc" {
			return true
		}
	}
	return false
}

func ensureSharedProtoc(ctx context.Context, entry ToolchainEntry) (string, error) {
	version := strings.TrimPrefix(strings.TrimSpace(entry.Version), "v")
	if version == "" {
		return "", fmt.Errorf("protoc toolchain entry missing version")
	}
	target := strings.TrimSpace(entry.Target)
	if target == "" {
		host, err := HostTriplet()
		if err != nil {
			return "", err
		}
		target = host
	}
	entry.Version = version
	entry.Target = target

	if ok, err := sharedProtocValid(entry); err != nil {
		return "", err
	} else if ok {
		return "", nil
	}

	dest := SharedProtocPath(version)
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", parent, err)
	}
	tmp, err := os.MkdirTemp(parent, "."+filepath.Base(dest)+".tmp-")
	if err != nil {
		return "", fmt.Errorf("create temp shared protoc dir: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmp)
		}
	}()

	if err := materializeProtoc(ctx, entry, tmp); err != nil {
		return "", err
	}
	if err := ensureSharedProtocExecutable(entry, tmp); err != nil {
		return "", err
	}
	if err := verifySharedProtoc(entry, tmp); err != nil {
		return "", err
	}
	manifest := sharedToolManifest{
		Name:        "protoc",
		Version:     version,
		Target:      target,
		SHA256:      strings.TrimSpace(entry.SHA256),
		Source:      sharedProtocSource(entry),
		InstalledAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeSharedToolManifest(tmp, manifest); err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", fmt.Errorf("replace %s: %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("install shared protoc %s: %w", dest, err)
	}
	cleanup = false
	return fmt.Sprintf("materialized shared protoc %s", version), nil
}

func materializeProtoc(ctx context.Context, entry ToolchainEntry, dest string) error {
	source := strings.TrimSpace(entry.Source)
	if source == "" {
		source = sharedProtocSource(entry)
	}
	if strings.HasPrefix(source, "file://") {
		u, err := url.Parse(source)
		if err != nil {
			return err
		}
		source = u.Path
	}
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		return copyProtocTree(source, dest)
	}

	archivePath, cleanup, err := fetchSharedArchive(ctx, source)
	if err != nil {
		return err
	}
	defer cleanup()
	return extractProtocZip(archivePath, dest)
}

func fetchSharedArchive(ctx context.Context, source string) (string, func(), error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		tmp, err := os.CreateTemp("", "op-shared-protoc-*.zip")
		if err != nil {
			return "", func() {}, err
		}
		cleanup := func() { _ = os.Remove(tmp.Name()) }
		defer func() { _ = tmp.Close() }()
		if err := download(ctx, source, tmp); err != nil {
			cleanup()
			return "", func() {}, err
		}
		return tmp.Name(), cleanup, nil
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return "", func() {}, err
	}
	return abs, func() {}, nil
}

func extractProtocZip(path, dest string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open protoc zip %s: %w", path, err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		name := filepath.ToSlash(file.Name)
		if !(strings.HasPrefix(name, "bin/") || strings.HasPrefix(name, "include/")) {
			continue
		}
		target, err := safeJoin(dest, name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, modePerm(file.FileInfo().Mode()))
		if err != nil {
			_ = in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = in.Close()
			_ = out.Close()
			return err
		}
		if err := in.Close(); err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

func copyProtocTree(source, dest string) error {
	for _, rel := range []string{"bin", "include"} {
		src := filepath.Join(source, rel)
		if _, err := os.Stat(src); err != nil {
			if rel == "include" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("shared protoc source missing %s: %w", src, err)
		}
		if err := copyTree(src, filepath.Join(dest, rel)); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(source, dest string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, modePerm(info.Mode()))
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, modePerm(info.Mode()))
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}

func sharedProtocValid(entry ToolchainEntry) (bool, error) {
	dest := SharedProtocPath(strings.TrimPrefix(strings.TrimSpace(entry.Version), "v"))
	return verifySharedProtoc(entry, dest) == nil, nil
}

func verifySharedProtoc(entry ToolchainEntry, root string) error {
	binary := SharedProtocBinaryIn(root, entry.Target)
	info, err := os.Stat(binary)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", binary)
	}
	if !strings.Contains(strings.TrimSpace(entry.Target), "windows") && info.Mode()&0o111 == 0 {
		return fmt.Errorf("%s is not executable", binary)
	}
	if strings.TrimSpace(entry.SHA256) == "" {
		return nil
	}
	actual, err := fileSHA256(binary)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, strings.TrimSpace(entry.SHA256)) {
		return fmt.Errorf("shared protoc sha256 mismatch for %s: got %s, want %s", binary, actual, entry.SHA256)
	}
	return nil
}

func ensureSharedProtocExecutable(entry ToolchainEntry, root string) error {
	if strings.Contains(strings.TrimSpace(entry.Target), "windows") {
		return nil
	}
	binary := SharedProtocBinaryIn(root, entry.Target)
	info, err := os.Stat(binary)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", binary)
	}
	mode := info.Mode().Perm()
	if mode&0o111 != 0 {
		return nil
	}
	return os.Chmod(binary, mode|0o111)
}

func writeSharedToolManifest(root string, manifest sharedToolManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(root, metadataFile), data, 0o644)
}

func sharedProtocSource(entry ToolchainEntry) string {
	if source := strings.TrimSpace(entry.Source); source != "" {
		return source
	}
	version := strings.TrimPrefix(strings.TrimSpace(entry.Version), "v")
	return fmt.Sprintf("https://github.com/protocolbuffers/protobuf/releases/download/v%s/protoc-%s-%s.zip",
		version, version, protocReleaseAsset(entry.Target))
}

func protocReleaseAsset(target string) string {
	switch strings.TrimSpace(target) {
	case "aarch64-apple-darwin":
		return "osx-aarch_64"
	case "x86_64-apple-darwin":
		return "osx-x86_64"
	case "x86_64-unknown-linux-gnu", "x86_64-unknown-linux-musl":
		return "linux-x86_64"
	case "aarch64-unknown-linux-gnu", "aarch64-unknown-linux-musl":
		return "linux-aarch_64"
	case "x86_64-windows-gnu", "x86_64-pc-windows-msvc":
		return "win64"
	default:
		return ""
	}
}

func SharedProtocPath(version string) string {
	return filepath.Join(SDKRoot(), "shared", "protoc", strings.TrimPrefix(strings.TrimSpace(version), "v"))
}

func SharedProtocBinaryIn(root, target string) string {
	name := "protoc"
	if strings.Contains(strings.TrimSpace(target), "windows") {
		name += ".exe"
	}
	return filepath.Join(root, "bin", name)
}

func SharedProtocBinary(version, target string) string {
	return SharedProtocBinaryIn(SharedProtocPath(version), target)
}

func SharedProtocInclude(version string) string {
	return filepath.Join(SharedProtocPath(version), "include")
}

func ProtocFromToolchain(entries []ToolchainEntry) (binary string, include string, ok bool, err error) {
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) != "protoc" {
			continue
		}
		version := strings.TrimPrefix(strings.TrimSpace(entry.Version), "v")
		target := strings.TrimSpace(entry.Target)
		if target == "" {
			target, err = HostTriplet()
			if err != nil {
				return "", "", false, err
			}
		}
		return SharedProtocBinary(version, target), SharedProtocInclude(version), true, nil
	}
	return "", "", false, nil
}
