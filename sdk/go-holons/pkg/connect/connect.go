// Package connect resolves holons to gRPC client connections.
package connect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
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
	"github.com/organic-programming/go-holons/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var ErrBinaryNotFound = errors.New("built binary not found")

type ConnectResult struct {
	Channel *grpc.ClientConn
	UID     string
	Origin  *discover.HolonRef
	Error   string
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

type runnerLaunchSpec struct {
	commandPath string
	argsPrefix  []string
}

var (
	mu      sync.Mutex
	started = map[*grpc.ClientConn]connHandle{}
)

func Connect(scope int, expression string, root *string, specifiers int, timeout int) ConnectResult {
	if scope != discover.LOCAL {
		return ConnectResult{Error: fmt.Sprintf("scope %d not supported", scope)}
	}

	target := strings.TrimSpace(expression)
	if target == "" {
		return ConnectResult{Error: "expression is required"}
	}

	if isHostPortTarget(target) {
		ctx, cancel := connectContext(timeout)
		defer cancel()

		conn, err := dialReady(ctx, normalizeDialTarget(target))
		if err != nil {
			return ConnectResult{Error: err.Error()}
		}

		origin := discover.HolonRef{URL: target}
		return ConnectResult{Channel: conn, Origin: &origin}
	}

	resolved := discover.Resolve(scope, target, root, specifiers, timeout)
	if resolved.Error != "" {
		return ConnectResult{Origin: resolved.Ref, Error: resolved.Error}
	}
	if resolved.Ref == nil {
		return ConnectResult{Error: fmt.Sprintf("holon %q not found", target)}
	}
	if resolved.Ref.Error != "" {
		return ConnectResult{Origin: resolved.Ref, Error: resolved.Ref.Error}
	}

	return connectResolved(*resolved.Ref, timeout)
}

func Disconnect(result ConnectResult) error {
	if result.Channel == nil {
		return nil
	}

	var handle connHandle
	var ok bool

	mu.Lock()
	handle, ok = started[result.Channel]
	delete(started, result.Channel)
	mu.Unlock()

	closeErr := result.Channel.Close()
	if !ok || handle.process == nil || !handle.ephemeral {
		return closeErr
	}

	stopErr := stopProcess(handle.process)
	if closeErr != nil {
		return closeErr
	}
	return stopErr
}

func connectResolved(ref discover.HolonRef, timeout int) ConnectResult {
	ctx, cancel := connectContext(timeout)
	defer cancel()

	if isReachableTarget(ref.URL) {
		conn, err := dialReady(ctx, normalizeDialTarget(ref.URL))
		if err == nil {
			refCopy := ref
			return ConnectResult{Channel: conn, Origin: &refCopy}
		}
	}

	origin := ref
	var lastErr error

	for _, listenURI := range launchListenURIs(ref) {
		cmd, err := launchCommandFromRef(ref, listenURI)
		if err != nil {
			lastErr = err
			continue
		}

		if listenURI == "stdio://" {
			conn, startedCmd, dialErr := grpcclient.DialStdioCommand(ctx, cmd)
			if dialErr != nil {
				lastErr = dialErr
				continue
			}
			remember(conn, connHandle{process: newProcessHandle(startedCmd), ephemeral: true})
			return ConnectResult{Channel: conn, Origin: &origin}
		}

		startedURI, proc, startErr := startAdvertisedHolon(ctx, cmd)
		if startErr != nil {
			lastErr = startErr
			continue
		}
		conn, dialErr := dialReady(ctx, normalizeDialTarget(startedURI))
		if dialErr != nil {
			_ = stopProcess(proc)
			lastErr = dialErr
			continue
		}

		origin.URL = startedURI
		remember(conn, connHandle{process: proc, ephemeral: true})
		return ConnectResult{Channel: conn, Origin: &origin}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("target unreachable")
	}
	return ConnectResult{Origin: &origin, Error: lastErr.Error()}
}

func launchCommandFromRef(ref discover.HolonRef, listenURI string) (*exec.Cmd, error) {
	entry, err := entryFromRef(ref)
	if err != nil {
		return nil, err
	}
	return launchCommand(entry, listenURI)
}

func entryFromRef(ref discover.HolonRef) (discover.HolonEntry, error) {
	if ref.Info == nil {
		return discover.HolonEntry{}, errors.New("holon metadata unavailable")
	}

	dir, err := pathFromFileURL(ref.URL)
	if err != nil {
		return discover.HolonEntry{}, err
	}
	actualPath := dir

	sourceKind := strings.TrimSpace(ref.Info.SourceKind)
	if sourceKind == "" {
		info, statErr := os.Stat(actualPath)
		switch {
		case statErr == nil && !info.IsDir():
			sourceKind = "binary"
		case strings.HasSuffix(strings.ToLower(actualPath), ".holon"):
			sourceKind = "package"
		default:
			sourceKind = "source"
		}
	}

	entryDir := actualPath
	entryBinary := strings.TrimSpace(ref.Info.Entrypoint)
	if sourceKind == "binary" {
		entryDir = filepath.Dir(actualPath)
		entryBinary = actualPath
	}

	entry := discover.HolonEntry{
		Slug:       ref.Info.Slug,
		UUID:       ref.Info.UUID,
		Dir:        entryDir,
		SourceKind: sourceKind,
		Runner:     ref.Info.Runner,
		Transport:  ref.Info.Transport,
		Entrypoint: entryBinary,
		Identity: identity.Identity{
			UUID:       ref.Info.UUID,
			GivenName:  ref.Info.Identity.GivenName,
			FamilyName: ref.Info.Identity.FamilyName,
			Motto:      ref.Info.Identity.Motto,
			Aliases:    append([]string(nil), ref.Info.Identity.Aliases...),
			Lang:       ref.Info.Lang,
			Status:     ref.Info.Status,
		},
		Manifest: &discover.Manifest{
			Kind: ref.Info.Kind,
			Build: discover.Build{
				Runner: ref.Info.Runner,
				Main:   ref.Info.BuildMain,
			},
			Artifacts: discover.Artifacts{
				Binary: entryBinary,
			},
		},
		Architectures: append([]string(nil), ref.Info.Architectures...),
		HasDist:       ref.Info.HasDist,
		HasSource:     ref.Info.HasSource,
	}
	if sourceKind == "package" {
		entry.PackageRoot = dir
	}
	return entry, nil
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
		target, ok := launchTargetForRunner("package-dist", entry.Runner, distEntrypoint, entry.Dir)
		if !ok {
			return launchTarget{}, fmt.Errorf("holon %q package dist is not runnable for runner %q", entry.Slug, entry.Runner)
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

	return launchTarget{}, fmt.Errorf("holon %q package is not runnable for arch %q", entry.Slug, archDir)
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

	holonPkgBin := filepath.Join(entry.Dir, ".op", "build",
		entry.Slug+".holon", "bin", packageArchDir(), filepath.Base(name))
	if info, err := os.Stat(holonPkgBin); err == nil && !info.IsDir() {
		return launchTarget{
			kind:             "source-built",
			commandPath:      holonPkgBin,
			workingDirectory: entry.Dir,
		}, nil
	}

	candidate := filepath.Join(entry.Dir, ".op", "build", "bin", filepath.Base(name))
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return launchTarget{
			kind:             "source-built",
			commandPath:      candidate,
			workingDirectory: entry.Dir,
		}, nil
	}

	runner := strings.ToLower(strings.TrimSpace(entry.Manifest.Build.Runner))
	mainPath := strings.TrimSpace(entry.Manifest.Build.Main)
	if target, ok := launchTargetForRunner("source-run", runner, mainPath, entry.Dir); ok {
		return target, nil
	}

	return launchTarget{}, fmt.Errorf("%w for holon %q", ErrBinaryNotFound, entry.Slug)
}

func sourceEntryFromPackageGit(entry discover.HolonEntry, gitRoot string) (discover.HolonEntry, error) {
	if entry.UUID != "" {
		resolved := discover.Resolve(discover.LOCAL, entry.UUID, &gitRoot, discover.SOURCE, discover.NO_TIMEOUT)
		if resolved.Ref != nil {
			return entryFromRef(*resolved.Ref)
		}
	}

	if entry.Slug != "" {
		resolved := discover.Resolve(discover.LOCAL, entry.Slug, &gitRoot, discover.SOURCE, discover.NO_TIMEOUT)
		if resolved.Ref != nil {
			return entryFromRef(*resolved.Ref)
		}
	}

	discovered := discover.Discover(discover.LOCAL, nil, &gitRoot, discover.SOURCE, 1, discover.NO_TIMEOUT)
	if discovered.Error != "" {
		return discover.HolonEntry{}, fmt.Errorf("discover package source for holon %q: %s", entry.Slug, discovered.Error)
	}
	if len(discovered.Found) == 0 {
		return discover.HolonEntry{}, fmt.Errorf("holon %q package git/ does not contain a runnable source holon", entry.Slug)
	}
	return entryFromRef(discovered.Found[0])
}

func packageArchDir() string {
	return runtime.GOOS + "_" + runtime.GOARCH
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

func connectContext(timeout int) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
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
	trimmed := strings.TrimSpace(target)
	if strings.HasPrefix(trimmed, "file://") {
		return false
	}
	if strings.Contains(trimmed, "://") {
		return true
	}
	return strings.Contains(trimmed, ":")
}

func isHostPortTarget(target string) bool {
	trimmed := strings.TrimSpace(target)
	return !strings.Contains(trimmed, "://") && strings.Contains(trimmed, ":")
}

func launchListenURIs(ref discover.HolonRef) []string {
	added := make(map[string]struct{})
	out := make([]string, 0, 4)
	add := func(uri string) {
		if strings.TrimSpace(uri) == "" {
			return
		}
		if _, ok := added[uri]; ok {
			return
		}
		added[uri] = struct{}{}
		out = append(out, uri)
	}

	if ref.Info != nil {
		add(launchURIForTransport(ref.Info.Transport, ref.Info.Slug))
	}

	add("stdio://")
	if runtime.GOOS != "windows" {
		add(launchURIForTransport("unix", transportSlug(ref)))
	}
	add(launchURIForTransport("tcp", transportSlug(ref)))
	add(launchURIForTransport("ws", transportSlug(ref)))
	add(launchURIForTransport("wss", transportSlug(ref)))
	return out
}

func transportSlug(ref discover.HolonRef) string {
	if ref.Info != nil && strings.TrimSpace(ref.Info.Slug) != "" {
		return ref.Info.Slug
	}
	return "holon"
}

func launchURIForTransport(transport string, slug string) string {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "", "default":
		return ""
	case "stdio":
		return "stdio://"
	case "tcp":
		return "tcp://127.0.0.1:0"
	case "unix":
		if runtime.GOOS == "windows" {
			return ""
		}
		return defaultUnixSocketURI(slug)
	case "ws":
		return "ws://127.0.0.1:0/grpc"
	case "wss":
		return "wss://127.0.0.1:0/grpc"
	default:
		return ""
	}
}

func isReachableTarget(target string) bool {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" || strings.HasPrefix(trimmed, "file://") {
		return false
	}
	if strings.Contains(trimmed, "://") {
		return true
	}
	return strings.Contains(trimmed, ":")
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

func pathFromFileURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("holon URL %q is not a local file target", raw)
	}
	path := parsed.Path
	if parsed.Host != "" && parsed.Host != "localhost" {
		path = "//" + parsed.Host + path
	}
	if path == "" {
		return "", fmt.Errorf("holon URL %q has no path", raw)
	}
	return filepath.Clean(path), nil
}

func defaultUnixSocketURI(slug string) string {
	return "unix://" + filepath.Join(os.TempDir(), fmt.Sprintf("holons-%s-%d.sock", socketLabel(slug), time.Now().UnixNano()))
}

func socketLabel(slug string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(slug)) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteByte('-')
		}
		if b.Len() >= 24 {
			break
		}
	}
	label := strings.Trim(b.String(), "-")
	if label == "" {
		return "holon"
	}
	return label
}

func startAdvertisedHolon(ctx context.Context, cmd *exec.Cmd) (string, *processHandle, error) {
	if cmd == nil {
		return "", nil, errors.New("start holon: command is required")
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
		if scanErr := scanner.Err(); scanErr != nil {
			errCh <- scanErr
		}
	}

	go scanPipe(bufio.NewScanner(stdout))
	go scanPipe(bufio.NewScanner(stderr))

	waitCh := proc.wait()
	for {
		select {
		case line := <-lineCh:
			if uri := firstURI(line); uri != "" && usableStartupURI(uri) {
				return uri, proc, nil
			}
		case scanErr := <-errCh:
			if scanErr != nil {
				_ = stopProcess(proc)
				return "", nil, fmt.Errorf("read startup output: %w", scanErr)
			}
		case waitErr := <-waitCh:
			if waitErr == nil {
				waitErr = errors.New("holon exited before advertising an address")
			}
			return "", nil, waitErr
		case <-ctx.Done():
			_ = stopProcess(proc)
			return "", nil, fmt.Errorf("timed out waiting for holon startup: %w", ctx.Err())
		}
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
