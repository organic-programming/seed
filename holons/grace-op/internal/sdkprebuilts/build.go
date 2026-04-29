package sdkprebuilts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// BuildOptions captures a request to build a SDK prebuilt from local sources.
type BuildOptions struct {
	Lang              string
	Target            string
	Version           string
	Jobs              int  // 0 = runtime default
	Force             bool // rebuild even if a cached tarball exists
	InstallAfterBuild bool // install resulting tarball into $OPPATH/sdk
	Stdout            io.Writer
	Stderr            io.Writer
}

// Build invokes the per-SDK build script under .github/scripts/ and lands
// the produced tarball into $OPPATH/sdk via the existing Install path so
// the on-disk layout is identical to a release-installed prebuilt.
//
// Build is the source-build counterpart of Install. They are explicit
// alternatives — no silent fallback.
func Build(ctx context.Context, opts BuildOptions) (Prebuilt, []string, error) {
	lang, err := NormalizeLang(opts.Lang)
	if err != nil {
		return Prebuilt{}, nil, err
	}
	target, err := NormalizeTarget(opts.Target)
	if err != nil {
		return Prebuilt{}, nil, err
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = defaultVersions[lang]
	}
	version, err = NormalizeVersion(version)
	if err != nil {
		return Prebuilt{}, nil, err
	}

	if reason := suspendedReason(lang, target); reason != "" {
		return Prebuilt{}, nil, fmt.Errorf("op sdk build %s --target %s is suspended: %s",
			lang, target, reason)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return Prebuilt{}, nil, fmt.Errorf("locate repo root: %w", err)
	}
	sourceTreeSHA, _, err := localSourceTreeSHA256(repoRoot, lang)
	if err != nil {
		return Prebuilt{}, nil, fmt.Errorf("hash SDK source tree: %w", err)
	}

	scriptPath := filepath.Join(repoRoot, ".github", "scripts", "build-prebuilt-"+lang+".sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return Prebuilt{}, nil, fmt.Errorf("build script not found at %s: %w", scriptPath, err)
	}

	// Pre-flight: surface known blockers before spending minutes invoking
	// the script only to have it fail with the same root cause.
	if blockers := collectCompileBlockers(repoRoot, lang, target); len(blockers) > 0 {
		return Prebuilt{}, nil, fmt.Errorf("%s prerequisites not met:\n  - %s",
			lang, strings.Join(blockers, "\n  - "))
	}

	tarballPath := filepath.Join(repoRoot, "dist", "sdk-prebuilts", lang, target,
		fmt.Sprintf("%s-holons-v%s-%s.tar.gz", lang, version, target))

	notes := []string{}
	if !opts.Force {
		if _, statErr := os.Stat(tarballPath); statErr == nil {
			notes = append(notes, fmt.Sprintf("cached tarball reused at %s", workspaceRel(repoRoot, tarballPath)))
			goto install
		}
	}

	if err := runBuildScript(ctx, scriptPath, lang, target, version, opts, repoRoot); err != nil {
		return Prebuilt{}, nil, err
	}

	if _, statErr := os.Stat(tarballPath); statErr != nil {
		return Prebuilt{}, nil, fmt.Errorf("build script %s exited 0 but produced no tarball at %s",
			workspaceRel(repoRoot, scriptPath), workspaceRel(repoRoot, tarballPath))
	}
	notes = append(notes, fmt.Sprintf("built tarball at %s", workspaceRel(repoRoot, tarballPath)))

install:
	if !opts.InstallAfterBuild {
		return Prebuilt{
			Lang:    lang,
			Version: version,
			Target:  target,
			Source:  tarballPath,
		}, notes, nil
	}

	prebuilt, installNotes, err := Install(ctx, InstallOptions{
		Lang:             lang,
		Target:           target,
		Version:          version,
		Source:           tarballPath,
		SourceTreeSHA256: sourceTreeSHA,
	})
	notes = append(notes, installNotes...)
	if err != nil {
		return Prebuilt{}, notes, err
	}
	return prebuilt, notes, nil
}

// LocalSourceTreeSHA256 returns a hash of the local SDK source tree and the
// matching build script. The file set is selected with git's standard ignore
// rules when available, matching the release manifest producer.
func LocalSourceTreeSHA256(lang string) (string, bool, error) {
	normalized, err := NormalizeLang(lang)
	if err != nil {
		return "", false, err
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return "", false, nil
	}
	return localSourceTreeSHA256(repoRoot, normalized)
}

func localSourceTreeSHA256(repoRoot, lang string) (string, bool, error) {
	sourceDir := filepath.Join(repoRoot, "sdk", lang+"-holons")
	info, err := os.Stat(sourceDir)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", true, err
	}
	if !info.IsDir() {
		return "", true, fmt.Errorf("%s is not a directory", sourceDir)
	}
	if _, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--git-dir").Output(); err != nil {
		return "", false, nil
	}

	scriptRel := filepath.ToSlash(filepath.Join(".github", "scripts", "build-prebuilt-"+lang+".sh"))
	sdkRel := filepath.ToSlash(filepath.Join("sdk", lang+"-holons"))
	files, err := sourceTreeFiles(repoRoot, sdkRel, scriptRel)
	if err != nil {
		return "", false, nil
	}
	if len(files) == 0 {
		return "", false, nil
	}
	return hashSourceTreeFiles(repoRoot, files)
}

func sourceTreeFiles(repoRoot string, roots ...string) ([]string, error) {
	args := []string{"-C", repoRoot, "ls-files", "-co", "--exclude-standard", "-z", "--"}
	args = append(args, roots...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("list SDK source files with git: %w", err)
	}
	return parseNULTerminatedPaths(out), nil
}

func parseNULTerminatedPaths(out []byte) []string {
	parts := bytes.Split(out, []byte{0})
	paths := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		path := filepath.ToSlash(string(part))
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func hashSourceTreeFiles(repoRoot string, paths []string) (string, bool, error) {
	h := sha256.New()
	for _, rel := range paths {
		path := filepath.Join(repoRoot, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(h, "%s\x00missing\x00", rel)
			continue
		}
		if err != nil {
			return "", true, err
		}
		if info.IsDir() {
			if object, ok := gitIndexObject(repoRoot, rel); ok {
				fmt.Fprintf(h, "%s\x00gitlink\x00%s\x00", rel, object)
				continue
			}
			fmt.Fprintf(h, "%s\x00dir\x00%o\x00", rel, info.Mode().Perm())
			continue
		}
		fmt.Fprintf(h, "%s\x00%s\x00%o\x00", rel, fileTypeTag(info.Mode()), info.Mode().Perm())
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return "", true, err
			}
			_, _ = h.Write([]byte(link))
			_, _ = h.Write([]byte{0})
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return "", true, err
		}
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return "", true, err
		}
		if err := f.Close(); err != nil {
			return "", true, err
		}
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), true, nil
}

func gitIndexObject(repoRoot, rel string) (string, bool) {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD:"+rel).Output()
	if err != nil {
		return "", false
	}
	object := strings.TrimSpace(string(out))
	if object == "" {
		return "", false
	}
	return object, true
}

func runBuildScript(ctx context.Context, scriptPath, lang, target, version string, opts BuildOptions, repoRoot string) error {
	env := append(os.Environ(),
		"SDK_TARGET="+target,
		"SDK_VERSION="+version,
	)
	if opts.Jobs > 0 {
		env = append(env, langJobsEnv(lang)+"="+strconv.Itoa(opts.Jobs))
	}
	if runtime.GOOS == "darwin" {
		if _, ok := os.LookupEnv("MACOSX_DEPLOYMENT_TARGET"); !ok {
			env = append(env, "MACOSX_DEPLOYMENT_TARGET=11.0")
		}
	}

	// Tee stderr to a tail buffer so we can include the last few lines in the
	// returned error without dumping the whole ninja log.
	tailBuf := newTailBuffer(40)
	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Dir = repoRoot
	cmd.Env = env
	cmd.Stdout = orDevNull(opts.Stdout)
	cmd.Stderr = io.MultiWriter(orDevNull(opts.Stderr), tailBuf)

	if err := cmd.Run(); err != nil {
		tail := strings.TrimSpace(tailBuf.String())
		if tail == "" {
			return fmt.Errorf("build script %s failed: %w", workspaceRel(repoRoot, scriptPath), err)
		}
		return fmt.Errorf("build script %s failed: %w\nlast stderr lines:\n%s",
			workspaceRel(repoRoot, scriptPath), err, tail)
	}
	return nil
}

// tailBuffer keeps the last N lines written to it. Used to capture the
// trailing portion of a long script's stderr for error reporting.
type tailBuffer struct {
	lines    []string
	maxLines int
	pending  bytes.Buffer
}

func newTailBuffer(maxLines int) *tailBuffer {
	return &tailBuffer{maxLines: maxLines}
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	n, _ := t.pending.Write(p)
	for {
		raw := t.pending.Bytes()
		idx := bytes.IndexByte(raw, '\n')
		if idx < 0 {
			break
		}
		line := string(raw[:idx])
		t.pending.Next(idx + 1)
		t.lines = append(t.lines, line)
		if len(t.lines) > t.maxLines {
			t.lines = t.lines[len(t.lines)-t.maxLines:]
		}
	}
	return n, nil
}

func (t *tailBuffer) String() string {
	if t.pending.Len() == 0 {
		return strings.Join(t.lines, "\n")
	}
	all := append([]string{}, t.lines...)
	all = append(all, t.pending.String())
	return strings.Join(all, "\n")
}

func langJobsEnv(lang string) string {
	return strings.ToUpper(lang) + "_HOLONS_JOBS"
}

func orDevNull(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

// ListCompilable reports which SDKs can be source-built on this checkout
// right now: the build script must exist, required submodules must be
// initialised, and required binaries must be on PATH. Each Prebuilt
// carries a Blockers list when something is missing.
func ListCompilable(langFilter string) ([]Prebuilt, []string, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return nil, nil, fmt.Errorf("locate repo root: %w", err)
	}

	host, err := HostTriplet()
	if err != nil {
		return nil, []string{fmt.Sprintf("host triplet unresolved: %v", err)}, nil
	}

	langs := make([]string, 0, len(defaultVersions))
	if strings.TrimSpace(langFilter) != "" {
		lang, err := NormalizeLang(langFilter)
		if err != nil {
			return nil, nil, err
		}
		langs = append(langs, lang)
	} else {
		for lang := range defaultVersions {
			langs = append(langs, lang)
		}
		sort.Strings(langs)
	}

	out := make([]Prebuilt, 0, len(langs))
	for _, lang := range langs {
		entry := Prebuilt{
			Lang:    lang,
			Target:  host,
			Version: defaultVersions[lang],
			Source:  filepath.Join(".github", "scripts", "build-prebuilt-"+lang+".sh"),
		}
		entry.Blockers = collectCompileBlockers(repoRoot, lang, host)
		out = append(out, entry)
	}
	return out, nil, nil
}

// suspendedReason returns the human-readable reason a (lang, target) pair is
// temporarily unsupported, or "" if the pair is fine. Used by Build (refuse)
// and ListCompilable (mark blocker).
func suspendedReason(lang, target string) string {
	if perTarget, ok := suspendedPrebuilts[lang]; ok {
		if reason, ok := perTarget[target]; ok {
			return reason
		}
	}
	return ""
}

// collectCompileBlockers returns human-readable reasons that op sdk build
// would fail right now for (lang, target). Empty slice means buildable.
func collectCompileBlockers(repoRoot, lang, target string) []string {
	var blockers []string

	if reason := suspendedReason(lang, target); reason != "" {
		blockers = append(blockers, "suspended: "+reason)
		return blockers
	}

	scriptPath := filepath.Join(repoRoot, ".github", "scripts", "build-prebuilt-"+lang+".sh")
	if _, err := os.Stat(scriptPath); err != nil {
		blockers = append(blockers, fmt.Sprintf("missing build script %s", workspaceRel(repoRoot, scriptPath)))
		return blockers
	}

	for _, sub := range submoduleMarkers(lang) {
		full := filepath.Join(repoRoot, sub)
		if _, err := os.Stat(full); err != nil {
			blockers = append(blockers,
				fmt.Sprintf("missing submodule marker %s — run `git submodule update --init --recursive`",
					workspaceRel(repoRoot, full)))
		}
	}

	for _, bin := range requiredBinaries(lang, target) {
		if _, err := exec.LookPath(bin); err != nil {
			blockers = append(blockers, fmt.Sprintf("%s not on PATH", bin))
		}
	}

	return blockers
}

// submoduleMarkers returns paths whose existence proves a required submodule
// was initialised. Mirrors what each build script errors out on early.
func submoduleMarkers(lang string) []string {
	switch lang {
	case "zig":
		return []string{
			"sdk/zig-holons/third_party/grpc/CMakeLists.txt",
			"sdk/zig-holons/third_party/protobuf-c/build-cmake/CMakeLists.txt",
		}
	case "c":
		return []string{
			"sdk/zig-holons/third_party/grpc/CMakeLists.txt",
			"sdk/zig-holons/third_party/protobuf-c/build-cmake/CMakeLists.txt",
			"sdk/cpp-holons/third_party/nlohmann-json",
		}
	case "cpp":
		return []string{
			"sdk/zig-holons/third_party/grpc/CMakeLists.txt",
			"sdk/cpp-holons/third_party/nlohmann-json",
		}
	case "ruby":
		return nil
	}
	return nil
}

// requiredBinaries returns the binaries the build-prebuilt-<lang>.sh script
// expects to find on PATH. Conservative — script may error earlier on its own.
func requiredBinaries(lang, target string) []string {
	switch lang {
	case "zig", "c", "cpp":
		bins := []string{"zig", "cmake", "ninja"}
		if strings.Contains(target, "apple-darwin") {
			bins = append(bins, "xcrun")
		}
		return bins
	case "ruby":
		return []string{"ruby", "bundle"}
	}
	return nil
}

// findRepoRoot walks up from CWD looking for go.work — the seed monorepo
// marker. Returns an error if none is found within 12 levels.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 12; i++ {
		if _, statErr := os.Stat(filepath.Join(dir, "go.work")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("repo root (go.work) not found in 12 ancestor directories of current dir")
}

func workspaceRel(repoRoot, path string) string {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}
	return rel
}
