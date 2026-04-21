package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	appName        = "op"
	metaPackageDir = "op"
)

type releaseTarget struct {
	GOOS       string
	GOARCH     string
	NPMDir     string
	NPMName    string
	WingetArch string
}

type releaseArtifact struct {
	Target          releaseTarget
	BinaryPath      string
	ArchivePath     string
	BinaryChecksum  string
	ArchiveChecksum string
}

var releaseTargets = []releaseTarget{
	{GOOS: "darwin", GOARCH: "amd64", NPMDir: "op-darwin-x64", NPMName: "@organic-programming/op-darwin-x64"},
	{GOOS: "darwin", GOARCH: "arm64", NPMDir: "op-darwin-arm64", NPMName: "@organic-programming/op-darwin-arm64"},
	{GOOS: "linux", GOARCH: "amd64", NPMDir: "op-linux-x64", NPMName: "@organic-programming/op-linux-x64"},
	{GOOS: "linux", GOARCH: "arm64", NPMDir: "op-linux-arm64", NPMName: "@organic-programming/op-linux-arm64"},
	{GOOS: "windows", GOARCH: "amd64", NPMDir: "op-win32-x64", NPMName: "@organic-programming/op-win32-x64", WingetArch: "x64"},
}

func main() {
	var (
		distDir = flag.String("dist", "dist", "output directory for release artifacts")
		repoRef = flag.String("repo", defaultRepository(), "GitHub repository in owner/name form")
		version = flag.String("version", "", "release version override (defaults to tag or local fallback)")
	)
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		fatalf("resolve working directory: %v", err)
	}
	distRoot := filepath.Join(root, *distDir)

	resolvedVersion, err := resolveVersion(root, *version)
	if err != nil {
		fatalf("resolve version: %v", err)
	}
	commit, err := resolveCommit(root)
	if err != nil {
		fatalf("resolve commit: %v", err)
	}

	if err := os.RemoveAll(distRoot); err != nil {
		fatalf("clear dist dir: %v", err)
	}
	if err := os.MkdirAll(distRoot, 0o755); err != nil {
		fatalf("create dist dir: %v", err)
	}

	fmt.Printf("Building %s %s from %s\n", appName, resolvedVersion, commit[:7])

	artifacts := make([]releaseArtifact, 0, len(releaseTargets))
	for _, target := range releaseTargets {
		artifact, buildErr := buildReleaseTarget(root, distRoot, resolvedVersion, commit, target)
		if buildErr != nil {
			fatalf("build %s/%s: %v", target.GOOS, target.GOARCH, buildErr)
		}
		artifacts = append(artifacts, artifact)
		fmt.Printf("  %s/%s -> %s\n", target.GOOS, target.GOARCH, relativeTo(root, artifact.ArchivePath))
	}

	if err := writeSHA256Sums(distRoot, artifacts); err != nil {
		fatalf("write SHA256SUMS: %v", err)
	}
	if err := writeHomebrewFormula(distRoot, *repoRef, resolvedVersion, artifacts); err != nil {
		fatalf("write Homebrew formula: %v", err)
	}
	if err := writeWingetManifest(distRoot, *repoRef, resolvedVersion, artifacts); err != nil {
		fatalf("write WinGet manifest: %v", err)
	}
	if err := stageNPMPackages(root, distRoot, resolvedVersion, artifacts); err != nil {
		fatalf("stage npm packages: %v", err)
	}
	if err := writeReleaseManifest(distRoot, *repoRef, resolvedVersion, commit, artifacts); err != nil {
		fatalf("write release manifest: %v", err)
	}

	fmt.Printf("Release artifacts ready in %s\n", relativeTo(root, distRoot))
}

func buildReleaseTarget(root, distRoot, version, commit string, target releaseTarget) (releaseArtifact, error) {
	stageDir := filepath.Join(distRoot, target.ID())
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return releaseArtifact{}, err
	}

	binaryPath := filepath.Join(stageDir, target.BinaryName())
	args := []string{
		"build",
		"-trimpath",
		"-ldflags",
		fmt.Sprintf("-s -w -X github.com/organic-programming/grace-op/api.Version=%s -X github.com/organic-programming/grace-op/api.Commit=%s", version, commit),
		"-o",
		binaryPath,
		"./cmd/op",
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
		"CGO_ENABLED=0",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return releaseArtifact{}, fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(output)))
	}
	if target.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0o755); err != nil {
			return releaseArtifact{}, err
		}
	}

	archivePath := filepath.Join(distRoot, target.ArchiveName())
	if err := archiveBinary(binaryPath, archivePath, target.BinaryName()); err != nil {
		return releaseArtifact{}, err
	}

	binaryChecksum, err := writeChecksum(binaryPath)
	if err != nil {
		return releaseArtifact{}, err
	}
	archiveChecksum, err := writeChecksum(archivePath)
	if err != nil {
		return releaseArtifact{}, err
	}

	return releaseArtifact{
		Target:          target,
		BinaryPath:      binaryPath,
		ArchivePath:     archivePath,
		BinaryChecksum:  binaryChecksum,
		ArchiveChecksum: archiveChecksum,
	}, nil
}

func archiveBinary(binaryPath, archivePath, entryName string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return archiveZip(binaryPath, archivePath, entryName)
	}
	return archiveTarGz(binaryPath, archivePath, entryName)
}

func archiveTarGz(binaryPath, archivePath, entryName string) error {
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	info, err := os.Stat(binaryPath)
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = entryName
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	in, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(tw, in)
	return err
}

func archiveZip(binaryPath, archivePath, entryName string) error {
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	info, err := os.Stat(binaryPath)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = entryName
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	in, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(writer, in)
	return err
}

func writeChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	line := fmt.Sprintf("%s  %s\n", sum, filepath.Base(path))
	if err := os.WriteFile(path+".sha256", []byte(line), 0o644); err != nil {
		return "", err
	}
	return sum, nil
}

func writeSHA256Sums(distRoot string, artifacts []releaseArtifact) error {
	lines := make([]string, 0, len(artifacts)*2)
	for _, artifact := range artifacts {
		lines = append(lines,
			fmt.Sprintf("%s  %s", artifact.BinaryChecksum, relativeTo(distRoot, artifact.BinaryPath)),
			fmt.Sprintf("%s  %s", artifact.ArchiveChecksum, relativeTo(distRoot, artifact.ArchivePath)),
		)
	}
	sort.Strings(lines)
	return os.WriteFile(filepath.Join(distRoot, "SHA256SUMS"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func writeHomebrewFormula(distRoot, repo, version string, artifacts []releaseArtifact) error {
	arm := artifactFor(artifacts, "darwin", "arm64")
	amd := artifactFor(artifacts, "darwin", "amd64")
	if arm == nil || amd == nil {
		return errors.New("missing darwin artifacts for Homebrew formula")
	}

	formula := fmt.Sprintf(`class Op < Formula
  desc "Organic Programming CLI"
  homepage "https://github.com/%s"
  version "%s"

  on_macos do
    if Hardware::CPU.arm?
      url "%s"
      sha256 "%s"
    else
      url "%s"
      sha256 "%s"
    end
  end

  def install
    bin.install "op"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/op version")
  end
end
`, repo, strings.TrimPrefix(version, "v"), releaseURL(repo, version, filepath.Base(arm.ArchivePath)), arm.ArchiveChecksum, releaseURL(repo, version, filepath.Base(amd.ArchivePath)), amd.ArchiveChecksum)

	formulaDir := filepath.Join(distRoot, "homebrew")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(formulaDir, "op.rb"), []byte(formula), 0o644)
}

func writeWingetManifest(distRoot, repo, version string, artifacts []releaseArtifact) error {
	windows := artifactFor(artifacts, "windows", "amd64")
	if windows == nil {
		return errors.New("missing windows artifact for WinGet manifest")
	}

	baseDir := filepath.Join(distRoot, "winget")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	versionNoV := strings.TrimPrefix(version, "v")
	releaseDate := time.Now().UTC().Format("2006-01-02")
	files := map[string]string{
		"OrganicProgramming.Op.yaml": fmt.Sprintf(`PackageIdentifier: OrganicProgramming.Op
PackageVersion: %s
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
`, versionNoV),
		"OrganicProgramming.Op.installer.yaml": fmt.Sprintf(`PackageIdentifier: OrganicProgramming.Op
PackageVersion: %s
Installers:
  - Architecture: %s
    InstallerType: zip
    NestedInstallerType: portable
    NestedInstallerFiles:
      - RelativeFilePath: %s
        PortableCommandAlias: op
    InstallerUrl: %s
    InstallerSha256: %s
ManifestType: installer
ManifestVersion: 1.6.0
`, versionNoV, windows.Target.WingetArch, windows.Target.BinaryName(), releaseURL(repo, version, filepath.Base(windows.ArchivePath)), strings.ToUpper(windows.ArchiveChecksum)),
		"OrganicProgramming.Op.locale.en-US.yaml": fmt.Sprintf(`PackageIdentifier: OrganicProgramming.Op
PackageVersion: %s
PackageLocale: en-US
Publisher: Organic Programming
PackageName: op
ShortDescription: Organic Programming CLI
ReleaseDate: %s
ManifestType: defaultLocale
ManifestVersion: 1.6.0
`, versionNoV, releaseDate),
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(baseDir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func stageNPMPackages(root, distRoot, version string, artifacts []releaseArtifact) error {
	sourceRoot := filepath.Join(root, "packaging", "npm")
	destRoot := filepath.Join(distRoot, "npm")
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := copyDir(filepath.Join(sourceRoot, entry.Name()), filepath.Join(destRoot, entry.Name())); err != nil {
			return err
		}
	}

	versionNoV := strings.TrimPrefix(version, "v")
	for _, artifact := range artifacts {
		packageDir := filepath.Join(destRoot, artifact.Target.NPMDir)
		if err := updatePackageJSON(filepath.Join(packageDir, "package.json"), versionNoV); err != nil {
			return err
		}
		binDir := filepath.Join(packageDir, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return err
		}
		destBinary := filepath.Join(binDir, artifact.Target.BinaryName())
		if err := copyFile(artifact.BinaryPath, destBinary); err != nil {
			return err
		}
		if artifact.Target.GOOS != "windows" {
			if err := os.Chmod(destBinary, 0o755); err != nil {
				return err
			}
		}
	}

	metaPath := filepath.Join(destRoot, metaPackageDir, "package.json")
	if err := updatePackageJSON(metaPath, versionNoV); err != nil {
		return err
	}
	var metaPackage map[string]any
	if err := readJSON(metaPath, &metaPackage); err != nil {
		return err
	}
	optional, ok := metaPackage["optionalDependencies"].(map[string]any)
	if !ok {
		return fmt.Errorf("meta package optionalDependencies missing or invalid")
	}
	for _, artifact := range artifacts {
		optional[artifact.Target.NPMName] = versionNoV
	}
	if err := writeJSON(metaPath, metaPackage); err != nil {
		return err
	}
	return nil
}

func writeReleaseManifest(distRoot, repo, version, commit string, artifacts []releaseArtifact) error {
	type manifestArtifact struct {
		GOOS           string `json:"goos"`
		GOARCH         string `json:"goarch"`
		Binary         string `json:"binary"`
		Archive        string `json:"archive"`
		BinarySHA256   string `json:"binary_sha256"`
		ArchiveSHA256  string `json:"archive_sha256"`
		NPMPackageName string `json:"npm_package_name"`
	}

	payload := struct {
		AppName    string             `json:"app_name"`
		Version    string             `json:"version"`
		Commit     string             `json:"commit"`
		Repository string             `json:"repository"`
		Artifacts  []manifestArtifact `json:"artifacts"`
	}{
		AppName:    appName,
		Version:    version,
		Commit:     commit,
		Repository: repo,
		Artifacts:  make([]manifestArtifact, 0, len(artifacts)),
	}

	for _, artifact := range artifacts {
		payload.Artifacts = append(payload.Artifacts, manifestArtifact{
			GOOS:           artifact.Target.GOOS,
			GOARCH:         artifact.Target.GOARCH,
			Binary:         relativeTo(distRoot, artifact.BinaryPath),
			Archive:        relativeTo(distRoot, artifact.ArchivePath),
			BinarySHA256:   artifact.BinaryChecksum,
			ArchiveSHA256:  artifact.ArchiveChecksum,
			NPMPackageName: artifact.Target.NPMName,
		})
	}
	sort.Slice(payload.Artifacts, func(i, j int) bool {
		if payload.Artifacts[i].GOOS == payload.Artifacts[j].GOOS {
			return payload.Artifacts[i].GOARCH < payload.Artifacts[j].GOARCH
		}
		return payload.Artifacts[i].GOOS < payload.Artifacts[j].GOOS
	})

	return writeJSON(filepath.Join(distRoot, "release.json"), payload)
}

func resolveVersion(root, override string) (string, error) {
	if value := strings.TrimSpace(override); value != "" {
		return normalizeVersion(value), nil
	}
	for _, envName := range []string{"VERSION", "GITHUB_REF_NAME"} {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" && strings.HasPrefix(value, "v") {
			return normalizeVersion(value), nil
		}
	}
	if value, err := gitOutput(root, "describe", "--tags", "--exact-match"); err == nil && value != "" {
		return normalizeVersion(value), nil
	}
	if value, err := defaultVersionFromMain(root); err == nil && value != "" {
		return normalizeVersion(value), nil
	}
	return "", fmt.Errorf("could not resolve version from tag, env, or cmd/op/main.go")
}

func resolveCommit(root string) (string, error) {
	if value := strings.TrimSpace(os.Getenv("GITHUB_SHA")); value != "" {
		return value, nil
	}
	return gitOutput(root, "rev-parse", "HEAD")
}

func defaultVersionFromMain(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "cmd", "op", "main.go"))
	if err != nil {
		return "", err
	}
	match := regexp.MustCompile(`version\s*=\s*"([^"]+)"`).FindSubmatch(data)
	if len(match) != 2 {
		return "", fmt.Errorf("version constant not found in cmd/op/main.go")
	}
	return string(match[1]), nil
}

func gitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func defaultRepository() string {
	if value := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")); value != "" {
		return value
	}
	return "organic-programming/grace-op"
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "refs/tags/") {
		value = strings.TrimPrefix(value, "refs/tags/")
	}
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	return value
}

func artifactFor(artifacts []releaseArtifact, goos, goarch string) *releaseArtifact {
	for i := range artifacts {
		if artifacts[i].Target.GOOS == goos && artifacts[i].Target.GOARCH == goarch {
			return &artifacts[i]
		}
	}
	return nil
}

func releaseURL(repo, version, file string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, file)
}

func relativeTo(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSON(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func updatePackageJSON(path, version string) error {
	var payload map[string]any
	if err := readJSON(path, &payload); err != nil {
		return err
	}
	payload["version"] = version
	return writeJSON(path, payload)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		if d.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}
		return copyFile(path, targetPath)
	})
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, in); err != nil {
		return err
	}
	if err := os.WriteFile(dst, buf.Bytes(), info.Mode()); err != nil {
		return err
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func (t releaseTarget) ID() string {
	return fmt.Sprintf("%s-%s-%s", appName, t.GOOS, t.GOARCH)
}

func (t releaseTarget) BinaryName() string {
	if t.GOOS == "windows" {
		return appName + ".exe"
	}
	return appName
}

func (t releaseTarget) ArchiveName() string {
	if t.GOOS == "windows" {
		return t.ID() + ".zip"
	}
	return t.ID() + ".tar.gz"
}
