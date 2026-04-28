package sdkprebuilts

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	openv "github.com/organic-programming/grace-op/internal/env"
)

const (
	metadataFile          = "manifest.json"
	releasesAPIEnv        = "OP_SDK_RELEASES_URL"
	defaultReleasesAPIURL = "https://api.github.com/repos/organic-programming/seed/releases"
)

var defaultVersions = map[string]string{
	"ruby": "1.58.3",
	"c":    "1.80.0",
	"cpp":  "1.80.0",
	"zig":  "0.1.0",
}

var allowedTargets = map[string]struct{}{
	"aarch64-apple-darwin":       {},
	"x86_64-apple-darwin":        {},
	"x86_64-unknown-linux-gnu":   {},
	"aarch64-unknown-linux-gnu":  {},
	"x86_64-unknown-linux-musl":  {},
	"aarch64-unknown-linux-musl": {},
	"x86_64-windows-gnu":         {},
	"x86_64-pc-windows-msvc":     {},
}

// suspendedPrebuilts marks specific (lang, target) pairs as temporarily
// unsupported. Different from allowedTargets, which is system-wide.
// Build() rejects these with a clear error; ListCompilable() reports them
// as blockers. To re-enable, remove the entry AND verify the build script
// case is valid for that target.
var suspendedPrebuilts = map[string]map[string]string{
	"ruby": {
		"x86_64-apple-darwin": "ruby SDK macOS Intel build is suspended pending toolchain fix",
	},
}

type Prebuilt struct {
	Lang          string   `json:"lang"`
	Version       string   `json:"version"`
	Target        string   `json:"target"`
	Path          string   `json:"path"`
	Source        string   `json:"source,omitempty"`
	ArchiveSHA256 string   `json:"archive_sha256,omitempty"`
	TreeSHA256    string   `json:"tree_sha256,omitempty"`
	Installed     bool     `json:"installed"`
	Blockers      []string `json:"blockers,omitempty"`
	Codegen       *Codegen `json:"codegen,omitempty"`
}

type Codegen struct {
	Plugins []CodegenPlugin `json:"plugins,omitempty"`
}

type CodegenPlugin struct {
	Name      string `json:"name"`
	Binary    string `json:"binary"`
	OutSubdir string `json:"out_subdir"`
}

type InstallOptions struct {
	Lang    string
	Target  string
	Version string
	Source  string
}

type QueryOptions struct {
	Lang    string
	Target  string
	Version string
}

func Install(ctx context.Context, opts InstallOptions) (Prebuilt, []string, error) {
	lang, err := NormalizeLang(opts.Lang)
	if err != nil {
		return Prebuilt{}, nil, err
	}
	target, err := NormalizeTarget(opts.Target)
	if err != nil {
		return Prebuilt{}, nil, err
	}
	source := strings.TrimSpace(opts.Source)
	version := strings.TrimSpace(opts.Version)
	if version != "" {
		version, err = NormalizeVersion(version)
		if err != nil {
			return Prebuilt{}, nil, err
		}
	}
	expectedSHA := ""
	if source == "" {
		available, err := resolveAvailable(ctx, QueryOptions{Lang: lang, Target: target, Version: version})
		if err != nil {
			return Prebuilt{}, nil, err
		}
		source = available.Source
		version = available.Version
		expectedSHA = available.ArchiveSHA256
	}

	archivePath, cleanup, err := fetchArchive(ctx, source)
	if err != nil {
		return Prebuilt{}, nil, err
	}
	defer cleanup()

	if expectedSHA == "" {
		expectedSHA, err = fetchExpectedSHA256(ctx, source, archivePath)
		if err != nil {
			return Prebuilt{}, nil, err
		}
	}
	actualSHA, err := fileSHA256(archivePath)
	if err != nil {
		return Prebuilt{}, nil, err
	}
	if !strings.EqualFold(expectedSHA, actualSHA) {
		return Prebuilt{}, nil, fmt.Errorf("sha256 mismatch for %s: got %s, want %s", source, actualSHA, expectedSHA)
	}

	if version == "" {
		version = inferVersionFromSource(source, lang, target)
	}
	if version == "" {
		version = defaultVersions[lang]
	}
	version, err = NormalizeVersion(version)
	if err != nil {
		return Prebuilt{}, nil, err
	}

	dest := InstallPath(lang, version, target)
	if existing, err := metadataForPath(dest); err == nil && strings.EqualFold(existing.ArchiveSHA256, actualSHA) {
		existing.Installed = true
		return existing, []string{"already installed"}, nil
	}

	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Prebuilt{}, nil, fmt.Errorf("create %s: %w", parent, err)
	}
	tmp, err := os.MkdirTemp(parent, "."+filepath.Base(dest)+".tmp-")
	if err != nil {
		return Prebuilt{}, nil, fmt.Errorf("create temp install dir: %w", err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.RemoveAll(tmp)
		}
	}()

	if err := extractTarGz(archivePath, tmp); err != nil {
		return Prebuilt{}, nil, err
	}
	archiveMetadata, err := metadataForPath(tmp)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Prebuilt{}, nil, err
	}
	treeSHA, err := treeSHA256(tmp)
	if err != nil {
		return Prebuilt{}, nil, err
	}

	prebuilt := Prebuilt{
		Lang:          lang,
		Version:       version,
		Target:        target,
		Path:          dest,
		Source:        source,
		ArchiveSHA256: actualSHA,
		TreeSHA256:    treeSHA,
		Installed:     true,
		Codegen:       archiveMetadata.Codegen,
	}
	if err := writeMetadata(tmp, prebuilt); err != nil {
		return Prebuilt{}, nil, err
	}
	if err := os.RemoveAll(dest); err != nil {
		return Prebuilt{}, nil, fmt.Errorf("replace %s: %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return Prebuilt{}, nil, fmt.Errorf("install %s: %w", dest, err)
	}
	cleanupTmp = false
	return prebuilt, nil, nil
}

func ListInstalled(langFilter string) ([]Prebuilt, error) {
	root := SDKRoot()
	langs := make([]string, 0, len(defaultVersions))
	if strings.TrimSpace(langFilter) != "" {
		lang, err := NormalizeLang(langFilter)
		if err != nil {
			return nil, err
		}
		langs = append(langs, lang)
	} else {
		for lang := range defaultVersions {
			langs = append(langs, lang)
		}
		sort.Strings(langs)
	}

	var out []Prebuilt
	for _, lang := range langs {
		langDir := filepath.Join(root, lang)
		versionEntries, err := os.ReadDir(langDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", langDir, err)
		}
		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}
			version := versionEntry.Name()
			versionDir := filepath.Join(langDir, version)
			targetEntries, err := os.ReadDir(versionDir)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", versionDir, err)
			}
			for _, targetEntry := range targetEntries {
				if !targetEntry.IsDir() {
					continue
				}
				target := targetEntry.Name()
				if _, ok := allowedTargets[target]; !ok {
					continue
				}
				path := filepath.Join(versionDir, target)
				prebuilt, err := metadataForPath(path)
				if err != nil {
					prebuilt = Prebuilt{
						Lang:      lang,
						Version:   version,
						Target:    target,
						Path:      path,
						Installed: true,
					}
				}
				prebuilt.Lang = lang
				prebuilt.Version = version
				prebuilt.Target = target
				prebuilt.Path = path
				prebuilt.Installed = true
				out = append(out, prebuilt)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lang != out[j].Lang {
			return out[i].Lang < out[j].Lang
		}
		if c := compareVersion(out[i].Version, out[j].Version); c != 0 {
			return c < 0
		}
		return out[i].Target < out[j].Target
	})
	return out, nil
}

func ListAvailable(langFilter string) ([]Prebuilt, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lang := ""
	if strings.TrimSpace(langFilter) != "" {
		normalized, err := NormalizeLang(langFilter)
		if err != nil {
			return nil, nil, err
		}
		lang = normalized
	}
	releases, err := fetchReleases(ctx)
	if err != nil {
		return nil, nil, err
	}
	entries, err := availableFromReleaseManifests(ctx, releases, lang, "", "")
	if err != nil {
		return nil, nil, err
	}
	if len(entries) == 0 {
		entries = availableFromReleases(releases, lang, "", "")
	}
	if len(entries) == 0 {
		return nil, []string{"no SDK prebuilt releases found"}, nil
	}
	return entries, nil, nil
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Draft   bool          `json:"draft"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releaseManifest struct {
	Schema    string                    `json:"schema"`
	SDK       string                    `json:"sdk"`
	Version   string                    `json:"version"`
	Tag       string                    `json:"tag"`
	Artifacts []releaseManifestArtifact `json:"artifacts"`
}

type releaseManifestArtifact struct {
	Target  string               `json:"target"`
	Archive releaseManifestFile  `json:"archive"`
	Debug   *releaseManifestFile `json:"debug,omitempty"`
	SBOM    *releaseManifestFile `json:"sbom,omitempty"`
}

type releaseManifestFile struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

func resolveAvailable(ctx context.Context, opts QueryOptions) (Prebuilt, error) {
	lang, err := NormalizeLang(opts.Lang)
	if err != nil {
		return Prebuilt{}, err
	}
	target, err := NormalizeTarget(opts.Target)
	if err != nil {
		return Prebuilt{}, err
	}
	version := strings.TrimSpace(opts.Version)
	if version != "" {
		version, err = NormalizeVersion(version)
		if err != nil {
			return Prebuilt{}, err
		}
	}
	releases, err := fetchReleases(ctx)
	if err != nil {
		return Prebuilt{}, err
	}
	entries, err := availableFromReleaseManifests(ctx, releases, lang, target, version)
	if err != nil {
		return Prebuilt{}, err
	}
	if len(entries) == 0 {
		entries = availableFromReleases(releases, lang, target, version)
	}
	if len(entries) == 0 {
		if version == "" {
			return Prebuilt{}, fmt.Errorf("no available sdk prebuilt release for %s %s", lang, target)
		}
		return Prebuilt{}, fmt.Errorf("no available sdk prebuilt release for %s %s %s", lang, version, target)
	}
	return entries[0], nil
}

func availableFromReleaseManifests(ctx context.Context, releases []githubRelease, langFilter, targetFilter, versionFilter string) ([]Prebuilt, error) {
	langFilter = strings.TrimSpace(langFilter)
	targetFilter = strings.TrimSpace(targetFilter)
	versionFilter = strings.TrimSpace(versionFilter)

	var out []Prebuilt
	for _, release := range releases {
		if release.Draft {
			continue
		}
		releaseLang, releaseVersion, ok := parseReleaseTag(release.TagName)
		if !ok {
			continue
		}
		if langFilter != "" && releaseLang != langFilter {
			continue
		}
		if versionFilter != "" && releaseVersion != versionFilter {
			continue
		}
		manifestAsset, ok := releaseManifestAsset(release)
		if !ok {
			continue
		}
		manifest, err := fetchReleaseManifest(ctx, manifestAsset.BrowserDownloadURL)
		if err != nil {
			return nil, err
		}
		if manifest.SDK != "" && manifest.SDK != releaseLang {
			return nil, fmt.Errorf("release manifest %s sdk = %q, want %q", manifestAsset.BrowserDownloadURL, manifest.SDK, releaseLang)
		}
		if manifest.Version != "" && manifest.Version != releaseVersion {
			return nil, fmt.Errorf("release manifest %s version = %q, want %q", manifestAsset.BrowserDownloadURL, manifest.Version, releaseVersion)
		}
		for _, artifact := range manifest.Artifacts {
			target := strings.TrimSpace(artifact.Target)
			if _, ok := allowedTargets[target]; !ok {
				continue
			}
			if targetFilter != "" && target != targetFilter {
				continue
			}
			source := strings.TrimSpace(artifact.Archive.URL)
			if source == "" {
				source = releaseAssetURL(release, artifact.Archive.Name)
			}
			if source == "" {
				continue
			}
			out = append(out, Prebuilt{
				Lang:          releaseLang,
				Version:       releaseVersion,
				Target:        target,
				Source:        source,
				ArchiveSHA256: strings.TrimSpace(artifact.Archive.SHA256),
				Installed:     false,
			})
		}
	}
	sortAvailable(out)
	return out, nil
}

func releaseManifestAsset(release githubRelease) (githubAsset, bool) {
	for _, asset := range release.Assets {
		if strings.TrimSpace(asset.Name) == "release-manifest.json" && strings.TrimSpace(asset.BrowserDownloadURL) != "" {
			return asset, true
		}
	}
	return githubAsset{}, false
}

func fetchReleaseManifest(ctx context.Context, source string) (releaseManifest, error) {
	var b strings.Builder
	if err := download(ctx, source, &b); err != nil {
		return releaseManifest{}, err
	}
	var manifest releaseManifest
	if err := json.Unmarshal([]byte(b.String()), &manifest); err != nil {
		return releaseManifest{}, fmt.Errorf("parse SDK release manifest %s: %w", source, err)
	}
	return manifest, nil
}

func releaseAssetURL(release githubRelease, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, asset := range release.Assets {
		if strings.TrimSpace(asset.Name) == name {
			return strings.TrimSpace(asset.BrowserDownloadURL)
		}
	}
	return ""
}

func fetchReleases(ctx context.Context) ([]githubRelease, error) {
	var b strings.Builder
	if err := download(ctx, releasesAPIURL(), &b); err != nil {
		return nil, err
	}
	var releases []githubRelease
	if err := json.Unmarshal([]byte(b.String()), &releases); err != nil {
		return nil, fmt.Errorf("parse SDK release list: %w", err)
	}
	return releases, nil
}

func releasesAPIURL() string {
	if override := strings.TrimSpace(os.Getenv(releasesAPIEnv)); override != "" {
		return override
	}
	return defaultReleasesAPIURL
}

func availableFromReleases(releases []githubRelease, langFilter, targetFilter, versionFilter string) []Prebuilt {
	langFilter = strings.TrimSpace(langFilter)
	targetFilter = strings.TrimSpace(targetFilter)
	versionFilter = strings.TrimSpace(versionFilter)

	var out []Prebuilt
	for _, release := range releases {
		if release.Draft {
			continue
		}
		releaseLang, releaseVersion, ok := parseReleaseTag(release.TagName)
		if !ok {
			continue
		}
		if langFilter != "" && releaseLang != langFilter {
			continue
		}
		if versionFilter != "" && releaseVersion != versionFilter {
			continue
		}
		for _, asset := range release.Assets {
			prebuilt, ok := parseReleaseAsset(asset, releaseLang, releaseVersion)
			if !ok {
				continue
			}
			if targetFilter != "" && prebuilt.Target != targetFilter {
				continue
			}
			out = append(out, prebuilt)
		}
	}
	sortAvailable(out)
	return out
}

func sortAvailable(out []Prebuilt) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lang != out[j].Lang {
			return out[i].Lang < out[j].Lang
		}
		if c := compareVersion(out[i].Version, out[j].Version); c != 0 {
			return c > 0
		}
		return out[i].Target < out[j].Target
	})
}

func parseReleaseTag(tag string) (string, string, bool) {
	for lang := range defaultVersions {
		prefix := lang + "-holons-v"
		if strings.HasPrefix(tag, prefix) {
			version := strings.TrimPrefix(tag, prefix)
			if version == "" {
				return "", "", false
			}
			return lang, version, true
		}
	}
	return "", "", false
}

func parseReleaseAsset(asset githubAsset, lang, version string) (Prebuilt, bool) {
	if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
		return Prebuilt{}, false
	}
	name := strings.TrimSpace(asset.Name)
	if strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".spdx.json") || strings.Contains(name, "-debug.tar.gz") {
		return Prebuilt{}, false
	}
	base := strings.TrimSuffix(name, ".tar.gz")
	if base == name {
		return Prebuilt{}, false
	}
	prefix := lang + "-holons-v" + version + "-"
	if !strings.HasPrefix(base, prefix) {
		return Prebuilt{}, false
	}
	target := strings.TrimPrefix(base, prefix)
	if _, ok := allowedTargets[target]; !ok {
		return Prebuilt{}, false
	}
	return Prebuilt{
		Lang:      lang,
		Version:   version,
		Target:    target,
		Source:    asset.BrowserDownloadURL,
		Installed: false,
	}, true
}

func Locate(opts QueryOptions) (Prebuilt, error) {
	lang, target, version, err := resolveQuery(opts)
	if err != nil {
		return Prebuilt{}, err
	}
	path := InstallPath(lang, version, target)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Prebuilt{}, fmt.Errorf("sdk prebuilt %s %s for %s is not installed", lang, version, target)
		}
		return Prebuilt{}, err
	}
	if !info.IsDir() {
		return Prebuilt{}, fmt.Errorf("sdk prebuilt path is not a directory: %s", path)
	}
	prebuilt, err := metadataForPath(path)
	if err != nil {
		prebuilt = Prebuilt{Lang: lang, Version: version, Target: target, Path: path}
	}
	prebuilt.Lang = lang
	prebuilt.Version = version
	prebuilt.Target = target
	prebuilt.Path = path
	prebuilt.Installed = true
	return prebuilt, nil
}

func Uninstall(opts QueryOptions) (Prebuilt, error) {
	prebuilt, err := Locate(opts)
	if err != nil {
		return Prebuilt{}, err
	}
	if err := os.RemoveAll(prebuilt.Path); err != nil {
		return Prebuilt{}, fmt.Errorf("remove %s: %w", prebuilt.Path, err)
	}
	prebuilt.Installed = false
	return prebuilt, nil
}

func Verify(opts QueryOptions) (Prebuilt, bool, error) {
	prebuilt, err := Locate(opts)
	if err != nil {
		return Prebuilt{}, false, err
	}
	if strings.TrimSpace(prebuilt.TreeSHA256) == "" {
		return prebuilt, false, fmt.Errorf("installed sdk prebuilt has no tree hash metadata: %s", prebuilt.Path)
	}
	actual, err := treeSHA256(prebuilt.Path)
	if err != nil {
		return prebuilt, false, err
	}
	if !strings.EqualFold(actual, prebuilt.TreeSHA256) {
		return prebuilt, false, fmt.Errorf("tree sha256 mismatch for %s: got %s, want %s", prebuilt.Path, actual, prebuilt.TreeSHA256)
	}
	return prebuilt, true, nil
}

func SDKRoot() string {
	return filepath.Join(openv.OPPATH(), "sdk")
}

// AllowedTargets returns the supported target triplets in deterministic order.
// Used by CLI completion and any caller that needs to enumerate targets.
func AllowedTargets() []string {
	out := make([]string, 0, len(allowedTargets))
	for t := range allowedTargets {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// DefaultVersion returns the pinned default version for a SDK lang and ok=true,
// or "" and ok=false for unknown langs. Caller should NormalizeLang first if
// the input may be uppercase or untrusted.
func DefaultVersion(lang string) (string, bool) {
	v, ok := defaultVersions[strings.ToLower(strings.TrimSpace(lang))]
	return v, ok
}

func InstallPath(lang, version, target string) string {
	return filepath.Join(SDKRoot(), strings.TrimSpace(lang), strings.TrimSpace(version), strings.TrimSpace(target))
}

func NormalizeLang(lang string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(lang))
	if _, ok := defaultVersions[normalized]; !ok {
		return "", fmt.Errorf("unsupported sdk prebuilt %q (supported: c, cpp, ruby, zig)", strings.TrimSpace(lang))
	}
	return normalized, nil
}

func NormalizeTarget(target string) (string, error) {
	normalized := strings.TrimSpace(target)
	if normalized == "" {
		host, err := HostTriplet()
		if err != nil {
			return "", err
		}
		normalized = host
	}
	if _, ok := allowedTargets[normalized]; !ok {
		return "", fmt.Errorf("unsupported sdk prebuilt target %q", normalized)
	}
	return normalized, nil
}

func NormalizeVersion(version string) (string, error) {
	normalized := strings.TrimSpace(version)
	if normalized == "" {
		return "", fmt.Errorf("version is required")
	}
	if normalized == "." || normalized == ".." || strings.ContainsAny(normalized, `/\`) {
		return "", fmt.Errorf("invalid sdk prebuilt version %q", version)
	}
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '+' || r == '-' {
			continue
		}
		return "", fmt.Errorf("invalid sdk prebuilt version %q", version)
	}
	return normalized, nil
}

func HostTriplet() (string, error) {
	return HostTripletFor(runtime.GOOS, runtime.GOARCH, isMuslLinux())
}

func HostTripletFor(goos, goarch string, musl bool) (string, error) {
	arch := ""
	switch goarch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	default:
		return "", fmt.Errorf("unsupported host architecture %q", goarch)
	}

	switch goos {
	case "darwin":
		return arch + "-apple-darwin", nil
	case "linux":
		libc := "gnu"
		if musl {
			libc = "musl"
		}
		return arch + "-unknown-linux-" + libc, nil
	case "windows":
		if arch != "x86_64" {
			return "", fmt.Errorf("unsupported Windows SDK prebuilt architecture %q", goarch)
		}
		return "x86_64-windows-gnu", nil
	default:
		return "", fmt.Errorf("unsupported host OS %q", goos)
	}
}

func resolveQuery(opts QueryOptions) (string, string, string, error) {
	lang, err := NormalizeLang(opts.Lang)
	if err != nil {
		return "", "", "", err
	}
	target, err := NormalizeTarget(opts.Target)
	if err != nil {
		return "", "", "", err
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version, err = latestInstalledVersion(lang, target)
		if err != nil {
			return "", "", "", err
		}
	} else {
		version, err = NormalizeVersion(version)
		if err != nil {
			return "", "", "", err
		}
	}
	return lang, target, version, nil
}

func latestInstalledVersion(lang, target string) (string, error) {
	langDir := filepath.Join(SDKRoot(), lang)
	entries, err := os.ReadDir(langDir)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("sdk prebuilt %s for %s is not installed", lang, target)
	}
	if err != nil {
		return "", err
	}
	var versions []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(langDir, entry.Name(), target)); err == nil && info.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("sdk prebuilt %s for %s is not installed", lang, target)
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersion(versions[i], versions[j]) > 0
	})
	return versions[0], nil
}

func metadataForPath(path string) (Prebuilt, error) {
	data, err := os.ReadFile(filepath.Join(path, metadataFile))
	if err != nil {
		return Prebuilt{}, err
	}
	var prebuilt Prebuilt
	if err := json.Unmarshal(data, &prebuilt); err != nil {
		return Prebuilt{}, fmt.Errorf("parse %s: %w", filepath.Join(path, metadataFile), err)
	}
	return prebuilt, nil
}

func writeMetadata(path string, prebuilt Prebuilt) error {
	data, err := json.MarshalIndent(prebuilt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(path, metadataFile), data, 0o644)
}

func fetchArchive(ctx context.Context, source string) (string, func(), error) {
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		tmp, err := os.CreateTemp("", "op-sdk-prebuilt-*.tar.gz")
		if err != nil {
			return "", func() {}, err
		}
		path := tmp.Name()
		cleanup := func() { _ = os.Remove(path) }
		defer func() { _ = tmp.Close() }()
		if err := download(ctx, trimmed, tmp); err != nil {
			cleanup()
			return "", func() {}, err
		}
		return path, cleanup, nil
	}
	if strings.HasPrefix(trimmed, "file://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return "", func() {}, err
		}
		trimmed = u.Path
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", func() {}, err
	}
	return abs, func() {}, nil
}

func fetchExpectedSHA256(ctx context.Context, source, archivePath string) (string, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		var b strings.Builder
		if err := download(ctx, source+".sha256", &b); err != nil {
			return "", err
		}
		return parseSHA256Line(b.String())
	}
	shaPath := archivePath + ".sha256"
	data, err := os.ReadFile(shaPath)
	if err != nil {
		return "", fmt.Errorf("read sha256 sidecar %s: %w", shaPath, err)
	}
	return parseSHA256Line(string(data))
}

func download(ctx context.Context, source string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: %s", source, resp.Status)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func parseSHA256Line(line string) (string, error) {
	fields := strings.Fields(line)
	for _, field := range fields {
		if len(field) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(field); err == nil {
			return strings.ToLower(field), nil
		}
	}
	return "", fmt.Errorf("sha256 sidecar does not contain a 64-character hex digest")
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractTarGz(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip %s: %w", path, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar %s: %w", path, err)
		}
		target, err := safeJoin(dest, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, dirModePerm(header.FileInfo().Mode())); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, modePerm(header.FileInfo().Mode()))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := validateRelativeLink(header.Linkname); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported tar entry %q in %s", header.Name, path)
		}
	}
}

func safeJoin(root, name string) (string, error) {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." {
		return root, nil
	}
	target := filepath.Join(root, cleanName)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".." && !filepath.IsAbs(rel)) {
		return target, nil
	}
	return "", fmt.Errorf("archive entry escapes install dir: %s", name)
}

func validateRelativeLink(link string) error {
	clean := filepath.Clean(filepath.FromSlash(link))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("archive symlink escapes install dir: %s", link)
	}
	return nil
}

func modePerm(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o644
	}
	return mode.Perm()
}

func dirModePerm(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o755
	}
	return mode.Perm()
}

func treeSHA256(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == metadataFile {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		fmt.Fprintf(h, "%s\x00%s\x00%o\x00", rel, fileTypeTag(info.Mode()), info.Mode().Perm())
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, _ = h.Write([]byte(link))
			_, _ = h.Write([]byte{0})
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		_, _ = h.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileTypeTag(mode os.FileMode) string {
	if mode&os.ModeSymlink != 0 {
		return "symlink"
	}
	return "file"
}

func inferVersionFromSource(source, lang, target string) string {
	base := filepath.Base(strings.TrimSpace(source))
	if strings.HasSuffix(base, ".tar.gz") {
		base = strings.TrimSuffix(base, ".tar.gz")
	}
	suffix := "-" + target
	releasePrefix := lang + "-holons-v"
	if strings.HasPrefix(base, releasePrefix) && strings.HasSuffix(base, suffix) {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(base, releasePrefix), suffix))
	}
	prefix := lang + "-"
	if strings.HasPrefix(base, prefix) && strings.HasSuffix(base, suffix) {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(base, prefix), suffix))
	}
	return ""
}

func compareVersion(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	max := len(as)
	if len(bs) > max {
		max = len(bs)
	}
	for i := 0; i < max; i++ {
		ai, bi := 0, 0
		if i < len(as) {
			ai, _ = strconv.Atoi(numericPrefix(as[i]))
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(numericPrefix(bs[i]))
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return strings.Compare(a, b)
}

func numericPrefix(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "0"
	}
	return b.String()
}

func isMuslLinux() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if matches, _ := filepath.Glob("/lib/ld-musl-*.so.1"); len(matches) > 0 {
		return true
	}
	if matches, _ := filepath.Glob("/usr/lib/ld-musl-*.so.1"); len(matches) > 0 {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ldd", "--version").CombinedOutput()
	if err == nil && strings.Contains(strings.ToLower(string(out)), "musl") {
		return true
	}
	return false
}
