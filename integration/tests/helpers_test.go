package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	defaultCommandTimeout = 5 * time.Minute
	processStartTimeout   = 2 * time.Minute
	shortTestReason       = "skipping in short mode"
)

var (
	opBinary    string
	seedRoot    string
	graceOpRoot string
)

type sandbox struct {
	Root     string
	OPPATH   string
	OPBIN    string
	CacheDir string
}

type runOptions struct {
	BinaryPath       string
	Context          context.Context
	DiscoverRoot     string
	Env              []string
	Input            string
	SkipDiscoverRoot bool
	Timeout          time.Duration
	WorkDir          string
}

type cmdResult struct {
	Args     []string
	Combined string
	Err      error
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
}

type processHandle struct {
	args     []string
	cmd      *exec.Cmd
	combined syncBuffer
	done     chan error
	stderr   syncBuffer
	stdout   syncBuffer
}

type lifecycleReport struct {
	Operation   string            `json:"operation"`
	Target      string            `json:"target"`
	Holon       string            `json:"holon"`
	Dir         string            `json:"dir"`
	Manifest    string            `json:"manifest"`
	Runner      string            `json:"runner,omitempty"`
	Kind        string            `json:"kind,omitempty"`
	Binary      string            `json:"binary,omitempty"`
	BuildTarget string            `json:"build_target,omitempty"`
	BuildMode   string            `json:"build_mode,omitempty"`
	Artifact    string            `json:"artifact,omitempty"`
	Commands    []string          `json:"commands,omitempty"`
	Notes       []string          `json:"notes,omitempty"`
	Children    []lifecycleReport `json:"children,omitempty"`
}

type installReport struct {
	Operation   string   `json:"operation"`
	Target      string   `json:"target"`
	Holon       string   `json:"holon"`
	Dir         string   `json:"dir,omitempty"`
	Manifest    string   `json:"manifest,omitempty"`
	Binary      string   `json:"binary,omitempty"`
	BuildTarget string   `json:"build_target,omitempty"`
	BuildMode   string   `json:"build_mode,omitempty"`
	Artifact    string   `json:"artifact,omitempty"`
	Installed   string   `json:"installed,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

type discoverJSON struct {
	Entries []struct {
		Slug         string `json:"slug"`
		RelativePath string `json:"relative_path"`
		Origin       string `json:"origin"`
	} `json:"entries"`
}

type listJSON struct {
	Entries []struct {
		Identity struct {
			UUID       string `json:"uuid"`
			GivenName  string `json:"givenName"`
			FamilyName string `json:"familyName"`
		} `json:"identity"`
		RelativePath string `json:"relativePath"`
		Origin       string `json:"origin"`
	} `json:"entries"`
}

type syncBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func TestMain(m *testing.M) {
	var cleanup func()
	var err error

	seedRoot, err = resolveSeedRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve seed root: %v\n", err)
		os.Exit(1)
	}
	graceOpRoot = filepath.Join(seedRoot, "holons", "grace-op")

	opBinary, cleanup, err = buildCanonicalBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build canonical op: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	os.Exit(m.Run())
}

func resolveSeedRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func buildCanonicalBinary() (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "op-integration-*")
	if err != nil {
		return "", nil, err
	}

	binaryName := "op"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/op")
	cmd.Dir = graceOpRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}

	return binaryPath, func() { _ = os.RemoveAll(tmpDir) }, nil
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()

	root := t.TempDir()
	oppath := filepath.Join(root, ".op")
	opbin := filepath.Join(oppath, "bin")
	cacheDir := filepath.Join(oppath, "cache")
	for _, dir := range []string{oppath, opbin, cacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	return &sandbox{
		Root:     root,
		OPPATH:   oppath,
		OPBIN:    opbin,
		CacheDir: cacheDir,
	}
}

func (s *sandbox) runOP(t *testing.T, args ...string) cmdResult {
	t.Helper()
	return s.runOPWithOptions(t, runOptions{}, args...)
}

func (s *sandbox) runOPWithOptions(t *testing.T, opts runOptions, args ...string) cmdResult {
	t.Helper()

	ctx := opts.Context
	if ctx == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = defaultCommandTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}

	fullArgs := commandArgs(opts, args...)
	binaryPath := opts.BinaryPath
	if strings.TrimSpace(binaryPath) == "" {
		binaryPath = opBinary
	}
	workDir := opts.WorkDir
	if strings.TrimSpace(workDir) == "" {
		workDir = seedRoot
	}

	cmd := exec.CommandContext(ctx, binaryPath, fullArgs...)
	cmd.Dir = workDir
	cmd.Env = s.commandEnv(opts.Env)
	if opts.Input != "" {
		cmd.Stdin = strings.NewReader(opts.Input)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := cmdResult{
		Args:     append([]string(nil), fullArgs...),
		Err:      err,
		ExitCode: exitCodeFor(err),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	result.Combined = result.Stdout + result.Stderr

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
	}

	return result
}

func (s *sandbox) startProcess(t *testing.T, opts runOptions, args ...string) *processHandle {
	t.Helper()

	fullArgs := commandArgs(opts, args...)
	binaryPath := opts.BinaryPath
	if strings.TrimSpace(binaryPath) == "" {
		binaryPath = opBinary
	}
	workDir := opts.WorkDir
	if strings.TrimSpace(workDir) == "" {
		workDir = seedRoot
	}

	cmd := exec.Command(binaryPath, fullArgs...)
	cmd.Dir = workDir
	cmd.Env = s.commandEnv(opts.Env)
	if opts.Input != "" {
		cmd.Stdin = strings.NewReader(opts.Input)
	}

	handle := &processHandle{
		args: append([]string(nil), fullArgs...),
		cmd:  cmd,
		done: make(chan error, 1),
	}
	cmd.Stdout = io.MultiWriter(&handle.stdout, &handle.combined)
	cmd.Stderr = io.MultiWriter(&handle.stderr, &handle.combined)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start %q: %v", strings.Join(fullArgs, " "), err)
	}

	go func() {
		handle.done <- cmd.Wait()
	}()

	return handle
}

func (s *sandbox) commandEnv(extra []string) []string {
	env := append([]string{}, os.Environ()...)
	pathValue := filteredPathEnv(os.Getenv("PATH"))
	if opBinary != "" {
		pathValue = filepath.Dir(opBinary) + string(os.PathListSeparator) + pathValue
	}
	env = withEnvValue(env, "PATH", pathValue)
	env = append(env,
		"OPPATH="+s.OPPATH,
		"OPBIN="+s.OPBIN,
	)
	env = append(env, extra...)
	return env
}

func commandArgs(opts runOptions, args ...string) []string {
	fullArgs := make([]string, 0, len(args)+2)
	if !opts.SkipDiscoverRoot {
		root := strings.TrimSpace(opts.DiscoverRoot)
		if root == "" {
			root = seedRoot
		}
		fullArgs = append(fullArgs, "--root", root)
	}
	fullArgs = append(fullArgs, args...)
	return fullArgs
}

func filteredPathEnv(pathValue string) string {
	if strings.TrimSpace(pathValue) == "" {
		return pathValue
	}

	entries := strings.Split(pathValue, string(os.PathListSeparator))
	filtered := make([]string, 0, len(entries))
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		cleaned := filepath.Clean(trimmed)
		lower := strings.ToLower(cleaned)
		if strings.Contains(lower, strings.ToLower(string(filepath.Separator)+".op"+string(filepath.Separator)+"bin")) {
			continue
		}
		if strings.HasSuffix(lower, strings.ToLower(filepath.Join(".op", "bin"))) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(filtered, string(os.PathListSeparator))
}

func withEnvValue(env []string, key string, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			out = append(out, prefix+value)
			replaced = true
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func (p *processHandle) Stop(t *testing.T) {
	t.Helper()
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}

	if runtime.GOOS == "windows" {
		_ = p.cmd.Process.Kill()
		_ = p.wait(10 * time.Second)
		return
	}

	_ = p.cmd.Process.Signal(os.Interrupt)
	if err := p.wait(3 * time.Second); err == nil {
		return
	}
	_ = p.cmd.Process.Kill()
	_ = p.wait(10 * time.Second)
}

func (p *processHandle) wait(timeout time.Duration) error {
	select {
	case err := <-p.done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for process")
	}
}

func (p *processHandle) waitForPattern(t *testing.T, pattern *regexp.Regexp, timeout time.Duration) string {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		combined := p.Combined()
		if match := pattern.FindStringSubmatch(combined); len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
		select {
		case err := <-p.done:
			t.Fatalf("process exited before pattern %q appeared: %v\nstdout:\n%s\nstderr:\n%s", pattern.String(), err, p.Stdout(), p.Stderr())
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %q\nstdout:\n%s\nstderr:\n%s", pattern.String(), p.Stdout(), p.Stderr())
	return ""
}

func (p *processHandle) waitForListenAddress(t *testing.T, timeout time.Duration) string {
	t.Helper()
	pattern := regexp.MustCompile(`gRPC server listening on ((?:tcp|unix)://\S+)`)
	return p.waitForPattern(t, pattern, timeout)
}

func (p *processHandle) Stdout() string {
	return p.stdout.String()
}

func (p *processHandle) Stderr() string {
	return p.stderr.String()
}

func (p *processHandle) Combined() string {
	return p.combined.String()
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return -1
	}
	return 1
}

func requireSuccess(t *testing.T, result cmdResult) {
	t.Helper()
	if result.Err != nil {
		t.Fatalf("command failed (exit=%d): %v\nargs: %s\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Err, strings.Join(result.Args, " "), result.Stdout, result.Stderr)
	}
}

func requireFailure(t *testing.T, result cmdResult) {
	t.Helper()
	if result.Err == nil {
		t.Fatalf("command unexpectedly succeeded\nargs: %s\nstdout:\n%s\nstderr:\n%s", strings.Join(result.Args, " "), result.Stdout, result.Stderr)
	}
}

func requireContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to contain %q", haystack, needle)
	}
}

func requirePathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path to exist: %s (%v)", path, err)
	}
}

func requirePathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected path to be missing: %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func decodeJSON[T any](t *testing.T, raw string) T {
	t.Helper()
	var value T
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("decode json: %v\npayload:\n%s", err, raw)
	}
	return value
}

func decodeJSONLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode json line: %v\nline=%s", err, line)
		}
		out = append(out, payload)
	}
	return out
}

func reportPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(seedRoot, filepath.FromSlash(path))
}

func buildReportFor(t *testing.T, sb *sandbox, slug string, extraArgs ...string) lifecycleReport {
	t.Helper()
	args := append([]string{"--format", "json", "build"}, extraArgs...)
	args = append(args, slug)
	result := sb.runOP(t, args...)
	requireSuccess(t, result)
	return decodeJSON[lifecycleReport](t, result.Stdout)
}

func buildDryRunReportFor(t *testing.T, sb *sandbox, slug string, extraArgs ...string) lifecycleReport {
	t.Helper()
	args := append([]string{"--format", "json", "build", "--dry-run"}, extraArgs...)
	args = append(args, slug)
	result := sb.runOP(t, args...)
	requireSuccess(t, result)
	return decodeJSON[lifecycleReport](t, result.Stdout)
}

func installReportFor(t *testing.T, sb *sandbox, args ...string) installReport {
	t.Helper()
	fullArgs := append([]string{"--format", "json", "install"}, args...)
	result := sb.runOP(t, fullArgs...)
	requireSuccess(t, result)
	return decodeJSON[installReport](t, result.Stdout)
}

func cleanHolon(t *testing.T, sb *sandbox, slug string) {
	t.Helper()
	result := sb.runOP(t, "clean", slug)
	requireSuccess(t, result)
}

func binaryPathFor(t *testing.T, sb *sandbox, slug string) string {
	t.Helper()
	report := buildDryRunReportFor(t, sb, slug)
	return reportPath(report.Binary)
}

func artifactPathFor(t *testing.T, sb *sandbox, slug string) string {
	t.Helper()
	report := buildDryRunReportFor(t, sb, slug)
	return reportPath(report.Artifact)
}

func removeArtifactFor(t *testing.T, sb *sandbox, slug string) {
	t.Helper()
	path := artifactPathFor(t, sb, slug)
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("remove artifact %s: %v", path, err)
	}
}

func installedNameFor(t *testing.T, sb *sandbox, slug string) string {
	t.Helper()
	report := installReportFor(t, sb, "--build", slug)
	return report.Installed
}

func readDiscoverJSON(t *testing.T, sb *sandbox) discoverJSON {
	t.Helper()
	result := sb.runOP(t, "--format", "json", "discover")
	requireSuccess(t, result)
	return decodeJSON[discoverJSON](t, result.Stdout)
}

func readListJSON(t *testing.T, sb *sandbox) listJSON {
	t.Helper()
	result := sb.runOP(t, "--format", "json", "list")
	requireSuccess(t, result)
	return decodeJSON[listJSON](t, result.Stdout)
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitUntil(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("condition not satisfied within %s", timeout)
}

func mcpConversation(t *testing.T, sb *sandbox, targets []string, requests []map[string]any) ([]map[string]any, cmdResult) {
	t.Helper()

	lines := make([]string, 0, len(requests))
	for _, req := range requests {
		payload, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal mcp request: %v", err)
		}
		lines = append(lines, string(payload))
	}
	input := strings.Join(lines, "\n") + "\n"

	args := append([]string{"mcp"}, targets...)
	result := sb.runOPWithOptions(t, runOptions{Input: input}, args...)
	return decodeJSONLines(t, result.Stdout), result
}

func skipIfShort(t *testing.T, reason string) {
	t.Helper()
	if testing.Short() {
		t.Skip(reason)
	}
}
