package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	DefaultCommandTimeout = 15 * time.Minute
	ProcessStartTimeout   = 15 * time.Minute
	ShortTestReason       = "skipping in short mode"
)

type runtimeState struct {
	once sync.Once

	err error

	opBinary          string
	seedRoot          string
	catalogueRoot     string
	artifactsBaseRoot string
	runsRoot          string
	toolCacheRoot     string
	sharedCacheRoot   string
	bootstrapRoot     string
	artifactsRoot     string
	tempBaseRoot      string
	tempAliasRoot     string
	workspaceRoot     string
	graceOpRoot       string
}

var sharedRuntime runtimeState

type Sandbox struct {
	Root     string
	OPPATH   string
	OPBIN    string
	CacheDir string
	TMPDIR   string
}

type RunOptions struct {
	BinaryPath       string
	Context          context.Context
	DiscoverRoot     string
	Env              []string
	Input            string
	SkipDiscoverRoot bool
	Timeout          time.Duration
	WorkDir          string
}

type CmdResult struct {
	Args     []string
	Combined string
	Err      error
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
}

type ProcessHandle struct {
	args     []string
	cmd      *exec.Cmd
	combined syncBuffer
	done     chan error
	stderr   syncBuffer
	stdout   syncBuffer
}

type syncBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func SeedRoot(t *testing.T) string {
	t.Helper()
	return mustRuntime(t).seedRoot
}

func DefaultWorkspaceDir(t *testing.T) string {
	t.Helper()
	rt := mustRuntime(t)
	if strings.TrimSpace(rt.workspaceRoot) != "" {
		return rt.workspaceRoot
	}
	return rt.seedRoot
}

func ShortSocketPath(t *testing.T, name string) string {
	t.Helper()

	rt := mustRuntime(t)
	base := rt.tempAliasRoot
	if strings.TrimSpace(base) == "" {
		base = rt.tempBaseRoot
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "sock"
	}
	trimmed = strings.NewReplacer("/", "-", ":", "-", ".", "-").Replace(trimmed)
	const prefixLimit = 8
	if len(trimmed) > prefixLimit {
		trimmed = trimmed[:prefixLimit]
	}

	sum := fnv.New32a()
	_, _ = sum.Write([]byte(t.Name()))
	_, _ = sum.Write([]byte{0})
	_, _ = sum.Write([]byte(name))
	unique := fmt.Sprintf("%s-%08x.sock", trimmed, sum.Sum32())

	path := filepath.Join(base, unique)
	_ = os.Remove(path)
	return path
}

func CanonicalOPBinary(t *testing.T) string {
	t.Helper()
	return mustRuntime(t).opBinary
}

func NewSandbox(t *testing.T) *Sandbox {
	t.Helper()

	rt := mustRuntime(t)
	root := artifactTempDir(t, rt, "sandboxes")
	oppath := filepath.Join(root, ".op")
	opbin := filepath.Join(oppath, "bin")
	cacheDir := filepath.Join(oppath, "cache")
	tmpDir := filepath.Join(root, "tmp")
	for _, dir := range []string{oppath, opbin, cacheDir, tmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	return &Sandbox{
		Root:     root,
		OPPATH:   oppath,
		OPBIN:    opbin,
		CacheDir: cacheDir,
		TMPDIR:   tmpDir,
	}
}

func (s *Sandbox) RunOP(t *testing.T, args ...string) CmdResult {
	t.Helper()
	return s.RunOPWithOptions(t, RunOptions{}, args...)
}

func (s *Sandbox) RunOPWithOptions(t *testing.T, opts RunOptions, args ...string) CmdResult {
	t.Helper()

	rt := mustRuntime(t)
	ctx := opts.Context
	if ctx == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = DefaultCommandTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}

	fullArgs := commandArgs(t, opts, args...)
	binaryPath := opts.BinaryPath
	if strings.TrimSpace(binaryPath) == "" {
		binaryPath = rt.opBinary
	}
	workDir := opts.WorkDir
	if strings.TrimSpace(workDir) == "" {
		workDir = DefaultWorkspaceDir(t)
	}

	cmd := exec.CommandContext(ctx, binaryPath, fullArgs...)
	cmd.Dir = workDir
	cmd.Env = s.commandEnv(t, opts.Env)
	if opts.Input != "" {
		cmd.Stdin = strings.NewReader(opts.Input)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := CmdResult{
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

func (s *Sandbox) StartProcess(t *testing.T, opts RunOptions, args ...string) *ProcessHandle {
	t.Helper()

	rt := mustRuntime(t)
	fullArgs := commandArgs(t, opts, args...)
	binaryPath := opts.BinaryPath
	if strings.TrimSpace(binaryPath) == "" {
		binaryPath = rt.opBinary
	}
	workDir := opts.WorkDir
	if strings.TrimSpace(workDir) == "" {
		workDir = DefaultWorkspaceDir(t)
	}

	cmd := exec.Command(binaryPath, fullArgs...)
	cmd.Dir = workDir
	cmd.Env = s.commandEnv(t, opts.Env)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if opts.Input != "" {
		cmd.Stdin = strings.NewReader(opts.Input)
	}

	handle := &ProcessHandle{
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

func (s *Sandbox) commandEnv(t *testing.T, extra []string) []string {
	rt := mustRuntime(t)
	env := append([]string{}, os.Environ()...)
	pathValue := filteredPathEnv(os.Getenv("PATH"))
	if rt.opBinary != "" {
		pathValue = filepath.Dir(rt.opBinary) + string(os.PathListSeparator) + pathValue
	}
	env = withEnvValue(env, "PATH", pathValue)
	env = append(env,
		"OPPATH="+s.OPPATH,
		"OPBIN="+s.OPBIN,
		"GOCACHE="+filepath.Join(rt.toolCacheRoot, "go-build"),
		"GOMODCACHE="+filepath.Join(rt.toolCacheRoot, "go-mod"),
		"GRACE_OP_SHARED_CACHE_DIR="+rt.sharedCacheRoot,
		"TMPDIR="+s.TMPDIR,
		"TMP="+s.TMPDIR,
		"TEMP="+s.TMPDIR,
	)
	env = append(env, extra...)
	return env
}

func commandArgs(t *testing.T, opts RunOptions, args ...string) []string {
	fullArgs := make([]string, 0, len(args)+2)
	if !opts.SkipDiscoverRoot {
		root := strings.TrimSpace(opts.DiscoverRoot)
		if root == "" {
			root = DefaultWorkspaceDir(t)
		}
		fullArgs = append(fullArgs, "--root", root)
	}
	fullArgs = append(fullArgs, args...)
	return fullArgs
}

func (p *ProcessHandle) Stop(t *testing.T) {
	t.Helper()
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}

	if runtime.GOOS == "windows" {
		_ = p.cmd.Process.Kill()
		_ = p.wait(10 * time.Second)
		return
	}

	if pgid, err := syscall.Getpgid(p.cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGINT)
		if err := p.wait(3 * time.Second); err == nil {
			return
		}
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
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

func (p *ProcessHandle) Signal(t *testing.T, sig os.Signal) {
	t.Helper()
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		t.Fatalf("cannot signal nil process")
	}
	if err := p.cmd.Process.Signal(sig); err != nil {
		t.Fatalf("signal process %v: %v", sig, err)
	}
}

func (p *ProcessHandle) WaitForListenAddress(t *testing.T, timeout time.Duration) string {
	t.Helper()
	pattern := regexp.MustCompile(`gRPC (?:server|bridge) listening on ((?:tcp|unix)://\S+)`)
	return p.waitForPattern(t, pattern, timeout)
}

func (p *ProcessHandle) WaitForCOAXListenAddress(t *testing.T, timeout time.Duration) string {
	t.Helper()
	pattern := regexp.MustCompile(`\[COAX\] server listening on ((?:tcp|unix)://\S+)`)
	return p.waitForPattern(t, pattern, timeout)
}

func (p *ProcessHandle) WaitForStdoutContains(t *testing.T, needle string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(p.Stdout(), needle) {
			return
		}
		select {
		case err := <-p.done:
			t.Fatalf("process exited before stdout contained %q: %v\nstdout:\n%s\nstderr:\n%s", needle, err, p.Stdout(), p.Stderr())
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for stdout to contain %q\nstdout:\n%s\nstderr:\n%s", needle, p.Stdout(), p.Stderr())
}

func (p *ProcessHandle) Wait(timeout time.Duration) error {
	if p == nil {
		return nil
	}
	return p.wait(timeout)
}

func (p *ProcessHandle) Stdout() string { return p.stdout.String() }

func (p *ProcessHandle) Stderr() string { return p.stderr.String() }

func (p *ProcessHandle) Combined() string { return p.combined.String() }

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

func RequireSuccess(t *testing.T, result CmdResult) {
	t.Helper()
	if result.Err != nil {
		t.Fatalf("command failed (exit=%d): %v\nargs: %s\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Err, strings.Join(result.Args, " "), result.Stdout, result.Stderr)
	}
}

func RequireFailure(t *testing.T, result CmdResult) {
	t.Helper()
	if result.Err == nil {
		t.Fatalf("command unexpectedly succeeded\nargs: %s\nstdout:\n%s\nstderr:\n%s", strings.Join(result.Args, " "), result.Stdout, result.Stderr)
	}
}

func RequireContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to contain %q", haystack, needle)
	}
}

func RequirePathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path to exist: %s (%v)", path, err)
	}
}

func RequirePathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected path to be missing: %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func DecodeJSON[T any](t *testing.T, raw string) T {
	t.Helper()
	var value T
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("decode json: %v\npayload:\n%s", err, raw)
	}
	return value
}

func DecodeJSONLines(t *testing.T, raw string) []map[string]any {
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

func AvailablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func WaitUntil(t *testing.T, timeout time.Duration, fn func() bool) {
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

func SkipIfShort(t *testing.T, reason string) {
	t.Helper()
	if testing.Short() {
		t.Skip(reason)
	}
}

func MCPConversation(t *testing.T, sb *Sandbox, targets []string, requests []map[string]any) ([]map[string]any, CmdResult) {
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
	result := sb.RunOPWithOptions(t, RunOptions{Input: input}, args...)
	return DecodeJSONLines(t, result.Stdout), result
}

func mustRuntime(t *testing.T) *runtimeState {
	t.Helper()
	sharedRuntime.once.Do(func() {
		sharedRuntime.err = initializeRuntime(&sharedRuntime)
	})
	if sharedRuntime.err != nil {
		t.Fatalf("initialize grace-op integration runtime: %v", sharedRuntime.err)
	}
	return &sharedRuntime
}

func initializeRuntime(rt *runtimeState) error {
	var err error
	rt.seedRoot, err = resolveSeedRoot()
	if err != nil {
		return err
	}
	rt.catalogueRoot = filepath.Join(rt.seedRoot, "ader", "catalogues", "grace-op")
	rt.artifactsBaseRoot = filepath.Join(rt.catalogueRoot, ".artifacts")
	rt.runsRoot = filepath.Join(rt.artifactsBaseRoot, "runs")
	rt.toolCacheRoot = filepath.Join(rt.artifactsBaseRoot, "tool-cache")
	rt.bootstrapRoot = filepath.Join(rt.artifactsBaseRoot, "bootstrap")
	rt.artifactsRoot = filepath.Join(rt.runsRoot, "run-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	// Slow SDK bootstraps (Ruby grpc gem, Swift packages, …) are keyed by their
	// own content-hash and must outlive any single test-binary invocation. Default
	// to a persistent path under artifactsBaseRoot, and honor an outer
	// GRACE_OP_SHARED_CACHE_DIR so an orchestrator (ader) can redirect it to a
	// catalogue-scoped cache.
	rt.sharedCacheRoot = filepath.Join(rt.artifactsBaseRoot, "shared")
	if outer := strings.TrimSpace(os.Getenv("GRACE_OP_SHARED_CACHE_DIR")); outer != "" {
		rt.sharedCacheRoot = outer
	}
	rt.tempBaseRoot = filepath.Join(os.TempDir(), "seed-int-store-"+strconv.Itoa(os.Getpid()))
	rt.graceOpRoot = filepath.Join(rt.seedRoot, "holons", "grace-op")

	if err := resetArtifactsRoot(rt); err != nil {
		return fmt.Errorf("prepare integration artifacts: %w", err)
	}
	rt.tempAliasRoot, err = ensureTempAlias(rt)
	if err != nil {
		return fmt.Errorf("prepare integration temp alias: %w", err)
	}
	rt.workspaceRoot, err = prepareWorkspaceMirror(rt)
	if err != nil {
		return fmt.Errorf("prepare mirrored workspace: %w", err)
	}
	rt.opBinary, err = buildCanonicalBinary(rt)
	if err != nil {
		return fmt.Errorf("build canonical op: %w", err)
	}
	return nil
}

func resolveSeedRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..")), nil
}

func buildCanonicalBinary(rt *runtimeState) (string, error) {
	binaryName := "op"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(rt.artifactsRoot, "bin", binaryName)
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/op")
	cmd.Dir = rt.graceOpRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = withEnvValue(cmd.Env, "GOCACHE", filepath.Join(rt.toolCacheRoot, "go-build"))
	cmd.Env = withEnvValue(cmd.Env, "GOMODCACHE", filepath.Join(rt.toolCacheRoot, "go-mod"))
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return binaryPath, nil
}

func resetArtifactsRoot(rt *runtimeState) error {
	if strings.TrimSpace(rt.artifactsRoot) == "" {
		return fmt.Errorf("artifacts root not set")
	}
	for _, dir := range []string{
		rt.artifactsBaseRoot,
		rt.runsRoot,
		rt.toolCacheRoot,
		rt.sharedCacheRoot,
		rt.bootstrapRoot,
		rt.artifactsRoot,
		filepath.Join(rt.artifactsRoot, "bin"),
		filepath.Join(rt.artifactsRoot, "sandboxes"),
		rt.tempBaseRoot,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func ensureTempAlias(rt *runtimeState) (string, error) {
	if strings.TrimSpace(rt.tempBaseRoot) == "" {
		return "", fmt.Errorf("temp alias target not set")
	}
	if runtime.GOOS == "windows" {
		return rt.tempBaseRoot, nil
	}

	aliasParent := "/tmp"
	if info, err := os.Stat(aliasParent); err != nil || !info.IsDir() {
		aliasParent = filepath.Dir(rt.tempBaseRoot)
	}
	alias := filepath.Join(aliasParent, "seed-int-"+strconv.Itoa(os.Getpid()))
	if err := os.RemoveAll(alias); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Symlink(rt.tempBaseRoot, alias); err != nil {
		return "", err
	}
	return alias, nil
}

func prepareWorkspaceMirror(rt *runtimeState) (string, error) {
	root := filepath.Join(rt.artifactsRoot, "workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	type mirrorSpec struct {
		src string
		dst string
	}
	for _, spec := range []mirrorSpec{
		{src: filepath.Join(rt.seedRoot, "examples"), dst: filepath.Join(root, "examples")},
		{src: filepath.Join(rt.seedRoot, "examples", "_protos"), dst: filepath.Join(root, "_protos")},
		{src: filepath.Join(rt.seedRoot, "holons", "grace-op", "_protos"), dst: filepath.Join(root, "_protos")},
		{src: filepath.Join(rt.seedRoot, "organism_kits"), dst: filepath.Join(root, "organism_kits")},
		{src: filepath.Join(rt.seedRoot, "protos"), dst: filepath.Join(root, "protos")},
		{src: filepath.Join(rt.seedRoot, "sdk"), dst: filepath.Join(root, "sdk")},
		{src: filepath.Join(rt.seedRoot, "holons"), dst: filepath.Join(root, "holons")},
		{src: filepath.Join(rt.seedRoot, "scripts"), dst: filepath.Join(root, "scripts")},
		{src: filepath.Join(rt.seedRoot, "ader", "catalogues", "grace-op", "integration"), dst: filepath.Join(root, "ader", "catalogues", "grace-op", "integration")},
		{src: filepath.Join(rt.seedRoot, "go.work"), dst: filepath.Join(root, "go.work")},
		{src: filepath.Join(rt.seedRoot, "go.work.sum"), dst: filepath.Join(root, "go.work.sum")},
	} {
		if _, err := os.Stat(spec.src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if err := copyTree(spec.src, spec.dst); err != nil {
			return "", fmt.Errorf("copy %s: %w", spec.src, err)
		}
	}
	if err := copyMirrorSupportProtos(rt, root); err != nil {
		return "", err
	}
	return root, nil
}

func copyMirrorSupportProtos(rt *runtimeState, root string) error {
	type protoAlias struct {
		dst string
		src string
	}
	candidates := []protoAlias{
		{
			dst: filepath.Join(root, "_protos", "google", "api", "annotations.proto"),
			src: filepath.Join(rt.seedRoot, "examples", "hello-world", "gabriel-greeting-node", "node_modules", "protobufjs", "google", "api", "annotations.proto"),
		},
		{
			dst: filepath.Join(root, "_protos", "google", "api", "http.proto"),
			src: filepath.Join(rt.seedRoot, "examples", "hello-world", "gabriel-greeting-node", "node_modules", "protobufjs", "google", "api", "http.proto"),
		},
		{
			dst: filepath.Join(root, "_protos", "validate", "validate.proto"),
			src: filepath.Join(rt.seedRoot, "examples", "hello-world", "gabriel-greeting-node", "node_modules", "@grpc", "grpc-js", "proto", "protoc-gen-validate", "validate", "validate.proto"),
		},
		{
			dst: filepath.Join(root, "_protos", "xds", "xds", "data", "orca", "v3", "orca_load_report.proto"),
			src: filepath.Join(rt.seedRoot, "examples", "hello-world", "gabriel-greeting-node", "node_modules", "@grpc", "grpc-js", "proto", "xds", "xds", "data", "orca", "v3", "orca_load_report.proto"),
		},
		{
			dst: filepath.Join(root, "_protos", "xds", "data", "orca", "v3", "orca_load_report.proto"),
			src: filepath.Join(rt.seedRoot, "examples", "hello-world", "gabriel-greeting-node", "node_modules", "@grpc", "grpc-js", "proto", "xds", "xds", "data", "orca", "v3", "orca_load_report.proto"),
		},
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate.src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		info, err := os.Stat(candidate.src)
		if err != nil {
			return err
		}
		if err := copyFile(candidate.src, candidate.dst, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(srcRoot, dstRoot string) error {
	return filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstRoot, rel)
		if shouldSkipMirrorPath(filepath.ToSlash(filepath.Join(filepath.Base(srcRoot), rel)), info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return err
			}
			return os.Symlink(target, dstPath)
		case info.IsDir():
			return os.MkdirAll(dstPath, info.Mode().Perm())
		default:
			return copyFile(path, dstPath, info.Mode())
		}
	})
}

func shouldSkipMirrorPath(rel string, info os.FileInfo) bool {
	if rel == "." {
		return false
	}
	base := info.Name()
	clean := strings.Trim(filepath.ToSlash(rel), "/")
	parts := strings.Split(clean, "/")
	if info.IsDir() {
		switch base {
		case ".git", ".gradle", ".kotlin", ".build", ".op", "build", "node_modules", "target", "__pycache__":
			return true
		}
		// Example builds can leave recursive install trees under examples/**/bin/default.
		// They are generated artifacts, not source inputs for mirrored integration workspaces.
		if base == "bin" && len(parts) >= 2 && parts[0] == "examples" {
			return true
		}
	}
	if len(parts) == 3 && parts[0] == "sdk" && parts[2] == "bin" && info.IsDir() {
		return true
	}
	return false
}

func copyFile(srcPath, dstPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	return dst.Close()
}

func artifactTempDir(t *testing.T, rt *runtimeState, category string) string {
	t.Helper()
	root := filepath.Join(rt.artifactsRoot, category)
	if category == "sandboxes" && strings.TrimSpace(rt.tempBaseRoot) != "" {
		root = filepath.Join(rt.tempBaseRoot, "sb")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir artifact temp root %s: %v", root, err)
	}
	dir, err := os.MkdirTemp(root, "tmp-")
	if err != nil {
		t.Fatalf("mkdtemp %s: %v", root, err)
	}
	return dir
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

func withEnvValue(env []string, key, value string) []string {
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

func (p *ProcessHandle) wait(timeout time.Duration) error {
	select {
	case err := <-p.done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for process")
	}
}

func (p *ProcessHandle) waitForPattern(t *testing.T, pattern *regexp.Regexp, timeout time.Duration) string {
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
