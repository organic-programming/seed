// Package connect resolves holons to gRPC client connections.
package connect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/discover"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

const defaultTimeout = 5 * time.Second

var ErrBinaryNotFound = errors.New("built binary not found")

const (
	// TransportAuto tries the platform-appropriate transport chain.
	TransportAuto = "auto"
	// TransportStdio forces an ephemeral stdio child process.
	TransportStdio = "stdio"
	// TransportUnix forces a Unix socket child process.
	TransportUnix = "unix"
	// TransportTCP forces a TCP child process.
	TransportTCP = "tcp"
)

const (
	// LifecycleEphemeral stops any spawned process when the connection is closed.
	LifecycleEphemeral = "ephemeral"
	// LifecyclePersistent keeps spawned daemons running and writes a reusable port file.
	LifecyclePersistent = "persistent"
)

// ConnectOptions control how slug resolution and startup behave.
type ConnectOptions struct {
	Timeout   time.Duration
	Transport string
	Lifecycle string
	Start     bool
	PortFile  string
}

type processHandle struct {
	cmd      *exec.Cmd
	waitOnce sync.Once
	waitCh   chan error
}

type connHandle struct {
	process   *processHandle
	ephemeral bool
}

type launchTarget struct {
	kind             string
	commandPath      string
	args             []string
	workingDirectory string
}

var (
	mu      sync.Mutex
	started = map[*grpc.ClientConn]connHandle{}
)

// Connect resolves a target and returns a ready gRPC connection.
func Connect(target string) (*grpc.ClientConn, error) {
	opts := ConnectOptions{
		Timeout:   defaultTimeout,
		Transport: TransportAuto,
		Lifecycle: LifecycleEphemeral,
		Start:     true,
	}
	return connectWithMode(target, opts)
}

// ConnectWithOpts resolves a target with explicit options.
func ConnectWithOpts(target string, opts ConnectOptions) (*grpc.ClientConn, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if strings.TrimSpace(opts.Transport) == "" {
		opts.Transport = TransportTCP
	}
	if strings.TrimSpace(opts.Lifecycle) == "" {
		opts.Lifecycle = LifecyclePersistent
	}
	return connectWithMode(target, opts)
}

// Disconnect closes a connection and stops any ephemeral process that connect started for it.
func Disconnect(conn *grpc.ClientConn) error {
	if conn == nil {
		return nil
	}

	var handle connHandle
	var ok bool

	mu.Lock()
	handle, ok = started[conn]
	delete(started, conn)
	mu.Unlock()

	closeErr := conn.Close()
	if !ok || handle.process == nil || !handle.ephemeral {
		return closeErr
	}

	stopErr := stopProcess(handle.process)
	if closeErr != nil {
		return closeErr
	}
	return stopErr
}

func connectWithMode(target string, opts ConnectOptions) (*grpc.ClientConn, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return nil, errors.New("target is required")
	}

	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}

	if isDirectTarget(trimmed) {
		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
		return dialReady(ctx, normalizeDialTarget(trimmed))
	}

	transport, err := normalizeTransport(opts.Transport)
	if err != nil {
		return nil, err
	}
	lifecycle, err := normalizeLifecycle(opts.Lifecycle)
	if err != nil {
		return nil, err
	}
	opts.Transport = transport
	opts.Lifecycle = lifecycle

	entry, err := discover.FindBySlug(trimmed)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("holon %q not found", trimmed)
	}

	portFile := strings.TrimSpace(opts.PortFile)
	if portFile == "" {
		portFile = defaultPortFilePath(entry.Slug)
	}

	if transportSupportsPortFileReuse(opts.Transport) {
		if uri, ok := usablePortFile(portFile, opts.Timeout, opts.Transport); ok {
			ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
			defer cancel()
			return dialReady(ctx, normalizeDialTarget(uri))
		}
	}

	transports := transportAttempts(opts.Transport, opts.Lifecycle)
	var errorsSeen []string
	binaryMissing := false
	for _, currentTransport := range transports {
		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		conn, err := connectViaTransport(ctx, trimmed, *entry, portFile, currentTransport, opts)
		cancel()
		if err != nil {
			if errors.Is(err, ErrBinaryNotFound) {
				binaryMissing = true
			}
			errorsSeen = append(errorsSeen, fmt.Sprintf("%s: %v", currentTransport, err))
			continue
		}
		return conn, nil
	}

	if len(errorsSeen) == 0 {
		return nil, fmt.Errorf("holon %q is not reachable", trimmed)
	}
	err = fmt.Errorf("connect %q failed: %s", trimmed, strings.Join(errorsSeen, "; "))
	if binaryMissing {
		return nil, fmt.Errorf("%w: %v", ErrBinaryNotFound, err)
	}
	return nil, err
}

func connectViaTransport(
	ctx context.Context,
	target string,
	entry discover.HolonEntry,
	portFile string,
	transport string,
	opts ConnectOptions,
) (*grpc.ClientConn, error) {
	switch transport {
	case TransportStdio:
		if opts.Lifecycle != LifecycleEphemeral {
			return nil, errors.New("stdio transport only supports ephemeral connect()")
		}
		if !opts.Start {
			return nil, fmt.Errorf("holon %q is not running", target)
		}
		cmd, err := launchCommand(entry, "stdio://")
		if err != nil {
			return nil, err
		}
		conn, startedCmd, err := grpcclient.DialStdioCommand(ctx, cmd)
		if err != nil {
			return nil, err
		}
		remember(conn, connHandle{process: newProcessHandle(startedCmd), ephemeral: true})
		return conn, nil
	case TransportUnix, TransportTCP:
		if !opts.Start {
			return nil, fmt.Errorf("holon %q is not running", target)
		}

		var (
			startedURI string
			proc       *processHandle
			err        error
		)
		switch transport {
		case TransportTCP:
			cmd, launchErr := launchCommand(entry, "tcp://127.0.0.1:0")
			if launchErr != nil {
				return nil, launchErr
			}
			startedURI, proc, err = startTCPHolon(ctx, cmd)
		case TransportUnix:
			socketURI := defaultUnixSocketURI(entry.Slug, portFile)
			cmd, launchErr := launchCommand(entry, socketURI)
			if launchErr != nil {
				return nil, launchErr
			}
			startedURI, proc, err = startUnixHolon(cmd, socketURI)
		}
		if err != nil {
			return nil, err
		}

		conn, err := dialReady(ctx, normalizeDialTarget(startedURI))
		if err != nil {
			_ = stopProcess(proc)
			return nil, err
		}

		ephemeral := opts.Lifecycle == LifecycleEphemeral
		if !ephemeral {
			if err := writePortFile(portFile, startedURI); err != nil {
				_ = stopProcess(proc)
				_ = conn.Close()
				return nil, err
			}
		}

		remember(conn, connHandle{process: proc, ephemeral: ephemeral})
		return conn, nil
	default:
		return nil, fmt.Errorf("unsupported transport %q", transport)
	}
}

func remember(conn *grpc.ClientConn, handle connHandle) {
	mu.Lock()
	started[conn] = handle
	mu.Unlock()
}

func dialReady(ctx context.Context, target string) (*grpc.ClientConn, error) {
	conn, err := grpcclient.Dial(ctx, target)
	if err != nil {
		return nil, err
	}
	if err := waitForReady(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func waitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	conn.Connect()
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return nil
		case connectivity.Shutdown:
			return errors.New("grpc connection shut down before becoming ready")
		}

		if !conn.WaitForStateChange(ctx, state) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("timed out waiting for gRPC readiness")
		}
	}
}

func usablePortFile(path string, timeout time.Duration, transport string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	target := strings.TrimSpace(string(data))
	if target == "" {
		_ = os.Remove(path)
		return "", false
	}
	if !portFileMatchesTransport(target, transport) {
		return "", false
	}

	checkTimeout := timeout / 4
	if checkTimeout <= 0 {
		checkTimeout = time.Second
	}
	if checkTimeout > time.Second {
		checkTimeout = time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	conn, err := dialReady(ctx, normalizeDialTarget(target))
	if err == nil {
		_ = conn.Close()
		return target, true
	}

	_ = os.Remove(path)
	return "", false
}

func normalizeTransport(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", TransportAuto:
		return TransportAuto, nil
	case TransportStdio:
		return TransportStdio, nil
	case TransportUnix:
		return TransportUnix, nil
	case TransportTCP:
		return TransportTCP, nil
	default:
		return "", fmt.Errorf("unsupported transport %q", value)
	}
}

func normalizeLifecycle(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", LifecycleEphemeral:
		return LifecycleEphemeral, nil
	case LifecyclePersistent:
		return LifecyclePersistent, nil
	default:
		return "", fmt.Errorf("unsupported lifecycle %q", value)
	}
}

func transportAttempts(transport string, lifecycle string) []string {
	if transport != TransportAuto {
		return []string{transport}
	}

	attempts := make([]string, 0, 3)
	if lifecycle == LifecycleEphemeral {
		attempts = append(attempts, TransportStdio)
	}
	if runtime.GOOS != "windows" {
		attempts = append(attempts, TransportUnix)
	}
	attempts = append(attempts, TransportTCP)
	return attempts
}

func transportSupportsPortFileReuse(transport string) bool {
	switch transport {
	case TransportAuto, TransportUnix, TransportTCP:
		return true
	default:
		return false
	}
}

func portFileMatchesTransport(target string, transport string) bool {
	trimmed := strings.TrimSpace(target)
	switch transport {
	case TransportAuto:
		return true
	case TransportTCP:
		return strings.HasPrefix(trimmed, "tcp://")
	case TransportUnix:
		return strings.HasPrefix(trimmed, "unix://")
	default:
		return false
	}
}

func startTCPHolon(ctx context.Context, cmd *exec.Cmd) (string, *processHandle, error) {
	if cmd == nil {
		return "", nil, errors.New("start tcp holon: command is required")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start %s: %w", commandLabel(cmd), err)
	}

	proc := newProcessHandle(cmd)

	lineCh := make(chan string, 16)
	errCh := make(chan error, 2)
	scanPipe := func(scanner *bufio.Scanner) {
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}

	go scanPipe(bufio.NewScanner(stdout))
	go scanPipe(bufio.NewScanner(stderr))

	waitCh := proc.wait()

	for {
		select {
		case line := <-lineCh:
			if uri := firstURI(line); uri != "" {
				if !usableStartupURI(uri) {
					continue
				}
				return uri, proc, nil
			}
		case err := <-errCh:
			if err != nil {
				_ = stopProcess(proc)
				return "", nil, fmt.Errorf("read startup output: %w", err)
			}
		case err := <-waitCh:
			if err == nil {
				err = errors.New("holon exited before advertising an address")
			}
			return "", nil, err
		case <-ctx.Done():
			_ = stopProcess(proc)
			return "", nil, fmt.Errorf("timed out waiting for holon startup: %w", ctx.Err())
		}
	}
}

func startUnixHolon(cmd *exec.Cmd, socketURI string) (string, *processHandle, error) {
	if cmd == nil {
		return "", nil, errors.New("start unix holon: command is required")
	}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start %s: %w", commandLabel(cmd), err)
	}

	return socketURI, newProcessHandle(cmd), nil
}

func launchCommand(entry discover.HolonEntry, listenURI string) (*exec.Cmd, error) {
	target, err := resolveLaunchTarget(entry)
	if err != nil {
		return nil, err
	}

	args := append(append([]string(nil), target.args...), "serve", "--listen", listenURI)
	cmd := exec.Command(target.commandPath, args...)
	cmd.Dir = target.workingDirectory
	return cmd, nil
}

func resolveLaunchTarget(entry discover.HolonEntry) (launchTarget, error) {
	if entry.SourceKind == "package" {
		if target, err := resolvePackageLaunchTarget(entry); err == nil {
			return target, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return launchTarget{}, err
		}
	}

	return resolveSourceLaunchTarget(entry)
}

func resolvePackageLaunchTarget(entry discover.HolonEntry) (launchTarget, error) {
	entrypoint := strings.TrimSpace(entry.Entrypoint)
	if entrypoint == "" && entry.Manifest != nil {
		entrypoint = strings.TrimSpace(entry.Manifest.Artifacts.Binary)
	}
	if entrypoint == "" {
		return launchTarget{}, fmt.Errorf("holon %q package has no entrypoint", entry.Slug)
	}

	archDir := packageArchDir()
	binaryPath := filepath.Join(entry.Dir, "bin", archDir, filepath.Base(entrypoint))
	if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
		return launchTarget{
			kind:             "package-bin",
			commandPath:      binaryPath,
			workingDirectory: entry.Dir,
		}, nil
	}

	distEntrypoint := filepath.Join(entry.Dir, "dist", filepath.FromSlash(entrypoint))
	if info, err := os.Stat(distEntrypoint); err == nil && !info.IsDir() {
		runner := entry.Runner
		if runner == "" && entry.Manifest != nil {
			runner = entry.Manifest.Build.Runner
		}
		target, ok := launchTargetForRunner("package-dist", runner, distEntrypoint, entry.Dir)
		if !ok {
			return launchTarget{}, fmt.Errorf("holon %q package dist is not runnable for runner %q", entry.Slug, runner)
		}
		return target, nil
	}

	gitRoot := filepath.Join(entry.Dir, "git")
	if info, err := os.Stat(gitRoot); err == nil && info.IsDir() {
		sourceEntry, sourceErr := sourceEntryFromPackageGit(entry, gitRoot)
		if sourceErr != nil {
			return launchTarget{}, sourceErr
		}
		return resolveSourceLaunchTarget(sourceEntry)
	}

	return launchTarget{}, fmt.Errorf("holon %q package is not runnable for arch %q: missing bin/%s/%s, dist/%s, and git/", entry.Slug, archDir, archDir, filepath.Base(entrypoint), filepath.ToSlash(entrypoint))
}

func resolveSourceLaunchTarget(entry discover.HolonEntry) (launchTarget, error) {
	if entry.Manifest == nil {
		return launchTarget{}, fmt.Errorf("holon %q has no manifest", entry.Slug)
	}

	name := strings.TrimSpace(entry.Manifest.Artifacts.Binary)
	if name == "" {
		return launchTarget{}, fmt.Errorf("holon %q has no artifacts.binary", entry.Slug)
	}

	if filepath.IsAbs(name) {
		if info, err := os.Stat(name); err == nil && !info.IsDir() {
			return launchTarget{
				kind:             "path",
				commandPath:      name,
				workingDirectory: entry.Dir,
			}, nil
		}
	}

	// Check the .holon package layout produced by op build:
	// .op/build/<slug>.holon/bin/<os_arch>/<binary>
	holonPkgBin := filepath.Join(entry.Dir, ".op", "build",
		entry.Slug+".holon", "bin", packageArchDir(), filepath.Base(name))
	if info, err := os.Stat(holonPkgBin); err == nil && !info.IsDir() {
		return launchTarget{
			kind:             "source-built",
			commandPath:      holonPkgBin,
			workingDirectory: entry.Dir,
		}, nil
	}

	// Legacy flat layout: .op/build/bin/<binary>
	candidate := filepath.Join(entry.Dir, ".op", "build", "bin", filepath.Base(name))
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return launchTarget{
			kind:             "source-built",
			commandPath:      candidate,
			workingDirectory: entry.Dir,
		}, nil
	}

	runner := ""
	mainPath := ""
	if entry.Manifest != nil {
		runner = strings.ToLower(strings.TrimSpace(entry.Manifest.Build.Runner))
		mainPath = strings.TrimSpace(entry.Manifest.Build.Main)
	}
	if target, ok := launchTargetForRunner("source-run", runner, mainPath, entry.Dir); ok {
		return target, nil
	}

	return launchTarget{}, fmt.Errorf("%w for holon %q", ErrBinaryNotFound, entry.Slug)
}

func sourceEntryFromPackageGit(entry discover.HolonEntry, gitRoot string) (discover.HolonEntry, error) {
	discovered, err := discover.Discover(gitRoot)
	if err != nil {
		return discover.HolonEntry{}, fmt.Errorf("discover package source for holon %q: %w", entry.Slug, err)
	}

	var fallback *discover.HolonEntry
	for i := range discovered {
		candidate := discovered[i]
		if candidate.SourceKind != "source" {
			continue
		}
		if fallback == nil {
			copy := candidate
			fallback = &copy
		}
		if entry.UUID != "" && candidate.UUID == entry.UUID {
			return candidate, nil
		}
		if candidate.Slug == entry.Slug {
			return candidate, nil
		}
	}
	if fallback != nil {
		return *fallback, nil
	}
	return discover.HolonEntry{}, fmt.Errorf("holon %q package git/ does not contain a runnable source holon", entry.Slug)
}

func packageArchDir() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}

type runnerLaunchSpec struct {
	commandPath string
	argsPrefix  []string
}

func launchTargetForRunner(kind string, runner string, entrypoint string, workingDirectory string) (launchTarget, bool) {
	trimmedEntrypoint := strings.TrimSpace(entrypoint)
	if trimmedEntrypoint == "" {
		return launchTarget{}, false
	}
	spec, ok := launchSpecForRunner(runner)
	if !ok {
		return launchTarget{}, false
	}
	args := append([]string(nil), spec.argsPrefix...)
	args = append(args, trimmedEntrypoint)
	return launchTarget{
		kind:             kind,
		commandPath:      spec.commandPath,
		args:             args,
		workingDirectory: workingDirectory,
	}, true
}

func launchSpecForRunner(runner string) (runnerLaunchSpec, bool) {
	switch strings.ToLower(strings.TrimSpace(runner)) {
	case "go", "go-module":
		return runnerLaunchSpec{commandPath: "go", argsPrefix: []string{"run"}}, true
	case "python":
		return runnerLaunchSpec{commandPath: "python3"}, true
	case "node", "typescript", "npm":
		return runnerLaunchSpec{commandPath: "node"}, true
	case "ruby":
		return runnerLaunchSpec{commandPath: "ruby"}, true
	case "dart":
		return runnerLaunchSpec{commandPath: "dart", argsPrefix: []string{"run"}}, true
	default:
		return runnerLaunchSpec{}, false
	}
}

func commandLabel(cmd *exec.Cmd) string {
	if cmd == nil {
		return ""
	}
	if len(cmd.Args) == 0 {
		return cmd.Path
	}
	return strings.Join(cmd.Args, " ")
}

func defaultPortFilePath(slug string) string {
	root, err := os.Getwd()
	if err != nil {
		root = "."
	}
	return filepath.Join(root, ".op", "run", slug+".port")
}

func defaultUnixSocketURI(slug string, portFile string) string {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(portFile))
	label := socketLabel(slug)
	return fmt.Sprintf("unix:///tmp/holons-%s-%012x.sock", label, hasher.Sum64()&0xffffffffffff)
}

func socketLabel(slug string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(slug)) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case (r == '-' || r == '_') && !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
		if b.Len() >= 24 {
			break
		}
	}

	label := strings.Trim(b.String(), "-")
	if label == "" {
		return "socket"
	}
	return label
}

func writePortFile(path, uri string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(uri)+"\n"), 0o644)
}

func newProcessHandle(cmd *exec.Cmd) *processHandle {
	return &processHandle{
		cmd:    cmd,
		waitCh: make(chan error, 1),
	}
}

func (p *processHandle) wait() <-chan error {
	if p == nil || p.cmd == nil {
		ch := make(chan error)
		close(ch)
		return ch
	}

	p.waitOnce.Do(func() {
		go func() {
			p.waitCh <- p.cmd.Wait()
			close(p.waitCh)
		}()
	})
	return p.waitCh
}

func stopProcess(proc *processHandle) error {
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return nil
	}

	if proc.cmd.ProcessState != nil && proc.cmd.ProcessState.Exited() {
		return nil
	}

	waitCh := proc.wait()
	if err := proc.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		_ = proc.cmd.Process.Kill()
		<-waitCh
		return err
	}

	select {
	case <-waitCh:
		return nil
	case <-time.After(2 * time.Second):
		if err := proc.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		<-waitCh
		return nil
	}
}

func isDirectTarget(target string) bool {
	if strings.Contains(target, "://") {
		return true
	}
	return strings.Contains(target, ":")
}

func normalizeDialTarget(target string) string {
	trimmed := strings.TrimSpace(target)
	if !strings.Contains(trimmed, "://") {
		return trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}

	switch parsed.Scheme {
	case "tcp":
		host := parsed.Hostname()
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		port := parsed.Port()
		if port == "" {
			return trimmed
		}
		return host + ":" + port
	case "unix":
		return trimmed
	default:
		return trimmed
	}
}

func firstURI(line string) string {
	for _, field := range strings.Fields(line) {
		trimmed := strings.TrimSpace(field)
		trimmed = strings.Trim(trimmed, "\"'()[]{}.,")
		if strings.HasPrefix(trimmed, "tcp://") ||
			strings.HasPrefix(trimmed, "unix://") ||
			strings.HasPrefix(trimmed, "ws://") ||
			strings.HasPrefix(trimmed, "wss://") ||
			strings.HasPrefix(trimmed, "stdio://") {
			return trimmed
		}
	}
	return ""
}

func usableStartupURI(uri string) bool {
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" {
		return false
	}
	if !strings.HasPrefix(trimmed, "tcp://") {
		return true
	}

	_, port, err := net.SplitHostPort(strings.TrimPrefix(trimmed, "tcp://"))
	return err == nil && port != "0"
}
