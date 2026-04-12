package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	sdkgrpcclient "github.com/organic-programming/go-holons/pkg/grpcclient"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	openv "github.com/organic-programming/grace-op/internal/env"
	internalgrpc "github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
	"github.com/organic-programming/grace-op/internal/progress"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Slow first-run backends such as Python/Ruby may need several minutes to
// install or initialize their runtime before they can advertise a transport.
const connectDispatchTimeout = 5 * time.Minute

const nativeStdioProbeTimeout = 20 * time.Second

type activeConnection struct {
	conn       *grpc.ClientConn
	disconnect func() error
	origin     *sdkdiscover.HolonRef
}

func (c activeConnection) close() error {
	if c.disconnect == nil {
		return nil
	}
	return c.disconnect()
}

type invokeExecutor func(index int, call invokeCall) (*internalgrpc.CallResult, error)

func emitInvokeResults(format Format, errPrefix string, calls []invokeCall, invoke invokeExecutor) int {
	for i, call := range calls {
		result, err := invoke(i, call)
		if err != nil {
			if len(calls) > 1 {
				fmt.Fprintf(os.Stderr, "%s: call %d/%d [%s]: %v\n", errPrefix, i+1, len(calls), call.method, err)
			} else {
				fmt.Fprintf(os.Stderr, "%s: %v\n", errPrefix, err)
			}
			return 1
		}
		if len(calls) == 1 {
			fmt.Println(formatRPCOutput(format, call.method, []byte(result.Output)))
			continue
		}
		fmt.Println(compactJSON(result.Output))
	}
	return 0
}

func runConnectedRPC(
	format Format,
	errPrefix string,
	holonName string,
	calls []invokeCall,
	transport string,
	noBuild bool,
) int {
	conn, err := connectForRPC(holonName, transport)
	if err != nil && !noBuild && isBuiltBinaryNotFound(err) {
		conn, err = autoBuildAndConnect(holonName, transport)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", errPrefix, err)
		return 1
	}
	defer func() { _ = conn.close() }()

	ctx, cancel := context.WithTimeout(context.Background(), connectDispatchTimeout)
	defer cancel()

	return emitInvokeResults(format, errPrefix, calls, func(_ int, call invokeCall) (*internalgrpc.CallResult, error) {
		result, callErr := internalgrpc.InvokeConn(ctx, conn.conn, call.method, call.inputJSON)
		if callErr != nil {
			if localResult, localErr := invokeConnViaLocalCatalog(ctx, conn.conn, holonName, call.method, call.inputJSON); localErr == nil {
				result = localResult
				callErr = nil
			}
		}
		return result, callErr
	})
}

// compactJSON returns a single-line JSON string with no extra whitespace.
// Falls back to the trimmed raw string when the input is not valid JSON.
func compactJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(trimmed)); err != nil {
		return trimmed
	}
	return buf.String()
}

func invokeConnViaLocalCatalog(ctx context.Context, conn *grpc.ClientConn, holonName string, method string, inputJSON string) (*internalgrpc.CallResult, error) {
	root := openv.Root()
	catalog, err := inspectpkg.LoadLocalWithOptions(holonName, &root, sdkdiscover.ALL, int(connectDispatchTimeout/time.Millisecond))
	if err == nil {
		for _, binding := range catalog.Methods {
			if strings.TrimSpace(binding.Method.Name) != strings.TrimSpace(method) {
				continue
			}
			return internalgrpc.InvokeMethodDescriptor(ctx, conn, binding.Descriptor, inputJSON)
		}
	}

	if builtin := builtinMethodDescriptor(holonName, method); builtin != nil {
		return internalgrpc.InvokeMethodDescriptor(ctx, conn, builtin, inputJSON)
	}

	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("method %q not found in local catalog for %s", method, holonName)
}

func builtinMethodDescriptor(holonName string, method string) protoreflect.MethodDescriptor {
	if !strings.EqualFold(strings.TrimSpace(holonName), "op") {
		return nil
	}

	service := opv1.File_api_v1_holon_proto.Services().ByName("OPService")
	if service == nil {
		return nil
	}

	targetMethod := canonicalMethodName(method)
	methods := service.Methods()
	for i := 0; i < methods.Len(); i++ {
		candidate := methods.Get(i)
		if string(candidate.Name()) == targetMethod {
			return candidate
		}
	}
	return nil
}

func parseConnectedRPCArgs(args []string) (calls []invokeCall, noBuild bool, err error) {
	if len(args) < 1 {
		return nil, false, fmt.Errorf("method required")
	}

	// Strip a leading --no-build flag before parsing method/payload pairs.
	remaining := args
	if len(remaining) > 0 && remaining[0] == "--no-build" {
		noBuild = true
		remaining = remaining[1:]
	}
	// Preserve the legacy single-call spelling: <method> --no-build [json].
	if !noBuild && len(remaining) > 1 && remaining[1] == "--no-build" {
		noBuild = true
		remaining = append([]string{remaining[0]}, remaining[2:]...)
	}
	for _, arg := range remaining[1:] {
		if strings.TrimSpace(arg) == "--no-build" {
			return nil, false, fmt.Errorf("--no-build must come immediately after the method")
		}
	}

	calls, err = parseInvokeCalls(remaining)
	if err != nil {
		return nil, false, err
	}
	return calls, noBuild, nil
}

func connectForRPC(holonName string, transport string) (activeConnection, error) {
	return connectForRPCWithTimeout(holonName, transport, connectDispatchTimeout)
}

func connectForRPCWithTimeout(holonName string, transport string, timeout time.Duration) (activeConnection, error) {
	root := openv.Root()

	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "", "auto":
		return connectBinaryAuto(holonName, &root, sdkdiscover.ALL, timeout)
	case "stdio":
		return connectForcedTransport(holonName, "stdio", &root, sdkdiscover.ALL, timeout)
	case "tcp":
		return connectForcedTransport(holonName, "tcp", &root, sdkdiscover.ALL, timeout)
	case "unix":
		return connectForcedTransport(holonName, "unix", &root, sdkdiscover.ALL, timeout)
	default:
		return connectBinaryAuto(holonName, &root, sdkdiscover.ALL, timeout)
	}
}

func connectBinaryAuto(holonName string, root *string, specifiers int, timeout time.Duration) (activeConnection, error) {
	resolved, binaryPath, err := resolveConnectedBinary(holonName, root, specifiers, timeout)
	if err != nil {
		return activeConnection{}, err
	}

	if conn, err := dialBinaryStdio(binaryPath, resolved.Ref, minDuration(timeout, nativeStdioProbeTimeout)); err == nil {
		return conn, nil
	}

	return connectLaunchedTCP(binaryPath, resolved.Ref, timeout)
}

func connectForcedTransport(holonName string, transport string, root *string, specifiers int, timeout time.Duration) (activeConnection, error) {
	if target, err := holons.ResolveTargetWithOptions(holonName, root, specifiers, int(timeout/time.Millisecond)); err == nil &&
		target != nil &&
		target.ManifestErr == nil &&
		target.Manifest != nil &&
		target.Manifest.Manifest.Kind == holons.KindComposite {
		return connectCompositeTarget(target, transport, timeout)
	}

	resolved, binaryPath, err := resolveConnectedBinary(holonName, root, specifiers, timeout)
	if err != nil {
		return activeConnection{}, err
	}

	switch transport {
	case "stdio":
		conn, err := dialBinaryStdio(binaryPath, resolved.Ref, minDuration(timeout, nativeStdioProbeTimeout))
		if err == nil {
			return conn, nil
		}
		return connectLaunchedTCP(binaryPath, resolved.Ref, timeout)
	case "tcp":
		return connectLaunchedTCP(binaryPath, resolved.Ref, timeout)
	case "unix":
		return connectLaunchedUnix(binaryPath, resolved.Ref, timeout)
	default:
		return activeConnection{}, fmt.Errorf("unsupported forced transport %q", transport)
	}
}

func resolveConnectedBinary(holonName string, root *string, specifiers int, timeout time.Duration) (sdkdiscover.ResolveResult, string, error) {
	resolved := holons.ResolveRef(holonName, root, specifiers, int(timeout/time.Millisecond))
	if resolved.Error != "" {
		if isResolveNotFound(resolved.Error) {
			if binaryPath, err := binaryPathForRef(nil, holonName); err == nil {
				return resolved, binaryPath, nil
			}
			return resolved, "", fmt.Errorf("%w for holon %q", sdkconnect.ErrBinaryNotFound, holonName)
		}
		return resolved, "", errors.New(resolved.Error)
	}
	if resolved.Ref == nil {
		if binaryPath, err := binaryPathForRef(nil, holonName); err == nil {
			return resolved, binaryPath, nil
		}
		return resolved, "", fmt.Errorf("%w for holon %q", sdkconnect.ErrBinaryNotFound, holonName)
	}

	binaryPath, err := binaryPathForRef(resolved.Ref, holonName)
	if err != nil {
		return resolved, "", err
	}
	return resolved, binaryPath, nil
}

func dialBinaryStdio(binaryPath string, origin *sdkdiscover.HolonRef, timeout time.Duration) (activeConnection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, cmd, err := sdkgrpcclient.DialStdio(ctx, binaryPath)
	if err != nil {
		return activeConnection{}, err
	}
	return activeConnection{
		conn: conn,
		disconnect: func() error {
			closeErr := conn.Close()
			killErr := stopCommand(cmd)
			if closeErr != nil {
				return closeErr
			}
			return killErr
		},
		origin: copyRef(origin),
	}, nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func binaryPathForRef(ref *sdkdiscover.HolonRef, fallback string) (string, error) {
	if ref != nil {
		if path, err := refLocalPath(ref); err == nil {
			if ref.Info != nil && strings.TrimSpace(ref.Info.SourceKind) == "binary" {
				return path, nil
			}
			if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
				return path, nil
			}
		}
	}

	binaryPath, err := holons.ResolveBinary(fallback)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return "", fmt.Errorf("%w for holon %q", sdkconnect.ErrBinaryNotFound, fallback)
		}
		return "", err
	}
	return binaryPath, nil
}

func connectLaunchedTCP(binaryPath string, origin *sdkdiscover.HolonRef, timeout time.Duration) (activeConnection, error) {
	return connectLaunchedAddress(binaryPath, origin, timeout, "tcp://127.0.0.1:0", nil)
}

func connectCompositeTarget(target *holons.Target, transport string, timeout time.Duration) (activeConnection, error) {
	if target == nil || target.Manifest == nil {
		return activeConnection{}, fmt.Errorf("composite target is missing its manifest")
	}

	switch transport {
	case "", "auto", "stdio":
		return connectCompositeTargetStdio(target, timeout)
	case "tcp":
		address, err := nextLoopbackAddress()
		if err != nil {
			return activeConnection{}, err
		}
		return connectLaunchedComposite(target, address, timeout)
	case "unix":
		if runtime.GOOS == "windows" {
			return activeConnection{}, fmt.Errorf("unix transport is not supported on windows")
		}
		socketDir, err := os.MkdirTemp("/tmp", "op-composite-")
		if err != nil {
			socketDir, err = os.MkdirTemp("", "op-composite-")
			if err != nil {
				return activeConnection{}, err
			}
		}
		socketPath := filepath.Join(socketDir, "h.sock")
		conn, err := connectLaunchedComposite(target, "unix://"+socketPath, timeout)
		if err != nil {
			_ = os.RemoveAll(socketDir)
			return activeConnection{}, err
		}
		previousDisconnect := conn.disconnect
		conn.disconnect = func() error {
			if previousDisconnect != nil {
				_ = previousDisconnect()
			}
			return os.RemoveAll(socketDir)
		}
		return conn, nil
	default:
		return activeConnection{}, fmt.Errorf("unsupported forced transport %q", transport)
	}
}

func connectCompositeTargetStdio(target *holons.Target, timeout time.Duration) (activeConnection, error) {
	address, err := nextLoopbackAddress()
	if err != nil {
		return activeConnection{}, err
	}
	conn, err := connectLaunchedComposite(target, address, timeout)
	if err == nil || !isCompositeStartupTimeout(err) {
		return conn, err
	}

	retryAddress, retryAddressErr := nextLoopbackAddress()
	if retryAddressErr != nil {
		return activeConnection{}, err
	}
	retryConn, retryErr := connectLaunchedComposite(target, retryAddress, timeout)
	if retryErr == nil {
		return retryConn, nil
	}
	return activeConnection{}, fmt.Errorf("%v; retry failed: %w", err, retryErr)
}

func connectLaunchedComposite(target *holons.Target, listenURI string, timeout time.Duration) (activeConnection, error) {
	artifactPath := strings.TrimSpace(target.Manifest.ArtifactPath(holons.BuildContext{}))
	if artifactPath == "" {
		return activeConnection{}, fmt.Errorf("composite artifact path is empty for %s", target.Manifest.Name)
	}

	effectiveListenURI, err := materializeCompositeListenURI(listenURI)
	if err != nil {
		return activeConnection{}, err
	}

	cmd, cleanup, err := compositeServeCommand(target.Manifest, artifactPath, effectiveListenURI)
	if err != nil {
		return activeConnection{}, err
	}

	var stdout strings.Builder
	var stderr strings.Builder
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		return activeConnection{}, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		return activeConnection{}, err
	}

	if err := cmd.Start(); err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		return activeConnection{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	addressCh := make(chan string, 1)
	readErrCh := make(chan error, 2)
	knownAddress := compositeKnownAddress(effectiveListenURI)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	streamReader := func(reader io.Reader, mirror *strings.Builder) {
		buffered := bufio.NewReader(reader)
		for {
			line, readErr := buffered.ReadString('\n')
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				mirror.WriteString(trimmed)
				mirror.WriteByte('\n')
				if address := advertisedListenAddress(trimmed); address != "" {
					select {
					case addressCh <- address:
					default:
					}
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					readErrCh <- nil
				} else {
					readErrCh <- readErr
				}
				return
			}
		}
	}

	go streamReader(stdoutPipe, &stdout)
	go streamReader(stderrPipe, &stderr)

	for {
		select {
		case address := <-addressCh:
			if conn := tryDialCompositeAddress(ctx, cmd, cleanup, normalizeDialTarget(address)); conn.conn != nil {
				return conn, nil
			}
		case <-ticker.C:
			if knownAddress == "" {
				continue
			}
			if conn := tryDialCompositeAddress(ctx, cmd, cleanup, normalizeDialTarget(knownAddress)); conn.conn != nil {
				return conn, nil
			}
		case readErr := <-readErrCh:
			if readErr == nil && cmd.ProcessState == nil {
				continue
			}
			if cleanup != nil {
				_ = cleanup()
			}
			if readErr != nil {
				return activeConnection{}, fmt.Errorf("composite server stream error: %w: %s%s", readErr, stdout.String(), stderr.String())
			}
			return activeConnection{}, fmt.Errorf("composite server exited before accepting connections on %s: %s%s", effectiveListenURI, stdout.String(), stderr.String())
		case <-ctx.Done():
			_ = stopCommand(cmd)
			if cleanup != nil {
				_ = cleanup()
			}
			return activeConnection{}, fmt.Errorf("composite server startup timeout for %s: %s%s", effectiveListenURI, stdout.String(), stderr.String())
		}
	}
}

func isCompositeStartupTimeout(err error) bool {
	return err != nil && strings.Contains(err.Error(), "composite server startup timeout")
}

func tryDialCompositeAddress(
	ctx context.Context,
	cmd *exec.Cmd,
	cleanup func() error,
	address string,
) activeConnection {
	conn, err := dialReadyAddress(ctx, address)
	if err != nil {
		return activeConnection{}
	}
	return activeConnection{
		conn: conn,
		disconnect: func() error {
			closeErr := conn.Close()
			killErr := stopCommand(cmd)
			if cleanup != nil {
				_ = cleanup()
			}
			if closeErr != nil {
				return closeErr
			}
			return killErr
		},
	}
}

func dialReadyAddress(ctx context.Context, address string) (*grpc.ClientConn, error) {
	trimmed := strings.TrimSpace(address)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}

	if strings.HasPrefix(trimmed, "unix://") {
		socketPath := strings.TrimPrefix(trimmed, "unix://")
		opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		}))
		trimmed = "passthrough:///unix"
	}

	//nolint:staticcheck // Blocking handshake is intentional for startup readiness.
	return grpc.DialContext(ctx, trimmed, opts...)
}

func compositeServeCommand(manifest *holons.LoadedManifest, artifactPath string, listenURI string) (*exec.Cmd, func() error, error) {
	trimmedArtifact := strings.TrimSpace(holons.LaunchableArtifactPath(artifactPath, manifest))
	if manifest == nil || trimmedArtifact == "" {
		return nil, nil, fmt.Errorf("composite artifact is not launchable")
	}

	bundleID := ""
	if isMacAppBundle(trimmedArtifact) {
		var err error
		bundleID, err = resolveCompositeBundleIdentifier(manifest, trimmedArtifact)
		if err != nil {
			return nil, nil, err
		}
	}

	tempHome, cleanup, err := writeCompositeCoaxPreferences(bundleID, listenURI)
	if err != nil {
		return nil, nil, err
	}

	if isMacAppBundle(trimmedArtifact) && runtime.GOOS == "darwin" {
		normalizeMacOSAppBundleMetadata(trimmedArtifact, manifest)
		executableName, err := appBundleInfoValue(
			filepath.Join(trimmedArtifact, "Contents", "Info.plist"),
			"CFBundleExecutable",
		)
		if err != nil {
			if cleanup != nil {
				_ = cleanup()
			}
			return nil, nil, err
		}
		executablePath := filepath.Join(
			trimmedArtifact,
			"Contents",
			"MacOS",
			strings.TrimSpace(executableName),
		)
		cmd := exec.Command(executablePath)
		cmd = withCompositeRunEnv(cmd, manifest)
		cmd = withCompositeCoaxEnv(cmd, listenURI)
		cmd.Dir = runCommandDir(nil, manifest, executablePath)
		cmd.Env = replaceCommandEnv(cmd.Env, "HOME", tempHome)
		cmd.Env = replaceCommandEnv(cmd.Env, "CFFIXED_USER_HOME", tempHome)
		cmd.Env = replaceCommandEnv(cmd.Env, "TMPDIR", filepath.Join(tempHome, "tmp"))
		if strings.TrimSpace(bundleID) != "" {
			cmd.Env = replaceCommandEnv(cmd.Env, "__CFBundleIdentifier", bundleID)
		}
		return cmd, cleanup, nil
	}

	cmd := exec.Command(trimmedArtifact)
	cmd = withCompositeRunEnv(cmd, manifest)
	cmd = withCompositeCoaxEnv(cmd, listenURI)
	cmd.Dir = runCommandDir(nil, manifest, trimmedArtifact)
	cmd.Env = replaceCommandEnv(cmd.Env, "HOME", tempHome)
	cmd.Env = replaceCommandEnv(cmd.Env, "XDG_CONFIG_HOME", filepath.Join(tempHome, ".config"))
	cmd.Env = replaceCommandEnv(cmd.Env, "APPDATA", filepath.Join(tempHome, "AppData", "Roaming"))
	cmd.Env = replaceCommandEnv(cmd.Env, "USERPROFILE", tempHome)
	cmd.Env = replaceCommandEnv(cmd.Env, "TMPDIR", filepath.Join(tempHome, "tmp"))
	return cmd, cleanup, nil
}

func withCompositeCoaxEnv(cmd *exec.Cmd, listenURI string) *exec.Cmd {
	if cmd == nil {
		return nil
	}
	env := cmd.Env
	if len(env) == 0 {
		env = os.Environ()
	}
	env = replaceCommandEnv(env, "OP_COAX_SERVER_ENABLED", "1")
	env = replaceCommandEnv(env, "OP_COAX_SERVER_LISTEN_URI", strings.TrimSpace(listenURI))
	cmd.Env = env
	return cmd
}

func writeCompositeCoaxPreferences(bundleID string, listenURI string) (string, func() error, error) {
	tempHome, err := os.MkdirTemp("", "op-composite-home-")
	if err != nil {
		return "", nil, err
	}

	transport, host, portText, unixPath := compositeListenSnapshot(listenURI)
	settings, err := json.Marshal(map[string]any{
		"serverEnabled":   true,
		"serverTransport": transport,
		"serverHost":      host,
		"serverPortText":  portText,
		"serverUnixPath":  unixPath,
		"relayEnabled":    false,
		"relayTransport":  "restSSE",
		"relayURL":        "",
		"mcpEnabled":      false,
		"mcpTransport":    "stdio",
		"mcpEndpoint":     "",
		"mcpCommand":      "",
	})
	if err != nil {
		_ = os.RemoveAll(tempHome)
		return "", nil, err
	}

	tmpDir := filepath.Join(tempHome, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		_ = os.RemoveAll(tempHome)
		return "", nil, err
	}

	if strings.TrimSpace(bundleID) == "" {
		return tempHome, func() error { return os.RemoveAll(tempHome) }, nil
	}

	prefsDir := filepath.Join(tempHome, "Library", "Preferences")
	if err := os.MkdirAll(prefsDir, 0o755); err != nil {
		_ = os.RemoveAll(tempHome)
		return "", nil, err
	}

	plistContents := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>coax.server.enabled</key>
  <true/>
  <key>coax.server.settings</key>
  <data>%s</data>
</dict>
</plist>
`, base64.StdEncoding.EncodeToString(settings))
	if err := os.WriteFile(filepath.Join(prefsDir, bundleID+".plist"), []byte(plistContents), 0o644); err != nil {
		_ = os.RemoveAll(tempHome)
		return "", nil, err
	}

	return tempHome, func() error { return os.RemoveAll(tempHome) }, nil
}

func resolveCompositeBundleIdentifier(manifest *holons.LoadedManifest, artifactPath string) (string, error) {
	if bundleID, err := appBundleInfoValue(filepath.Join(artifactPath, "Contents", "Info.plist"), "CFBundleIdentifier"); err == nil {
		bundleID = strings.TrimSpace(bundleID)
		if bundleID != "" && !strings.Contains(bundleID, "$(") {
			return bundleID, nil
		}
	}

	if manifest != nil {
		if bundleID := xcodeBuildSettingValue(filepath.Join(manifest.Dir, "project.yml"), "PRODUCT_BUNDLE_IDENTIFIER"); bundleID != "" {
			return bundleID, nil
		}
		if matches, err := filepath.Glob(filepath.Join(manifest.Dir, "*.xcodeproj", "project.pbxproj")); err == nil {
			for _, path := range matches {
				if bundleID := xcodeBuildSettingValue(path, "PRODUCT_BUNDLE_IDENTIFIER"); bundleID != "" {
					return bundleID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("bundle identifier not found for composite %s", artifactPath)
}

func xcodeBuildSettingValue(path string, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	yamlNeedle := key + ":"
	pbxNeedle := key + " ="
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(line, yamlNeedle):
			value := strings.TrimSpace(strings.TrimPrefix(line, yamlNeedle))
			return strings.Trim(strings.TrimSuffix(value, ";"), `"`)
		case strings.Contains(line, pbxNeedle):
			index := strings.Index(line, pbxNeedle)
			if index < 0 {
				continue
			}
			value := strings.TrimSpace(line[index+len(pbxNeedle):])
			return strings.Trim(strings.TrimSuffix(value, ";"), `"`)
		}
	}

	return ""
}

func compositeListenSnapshot(listenURI string) (transport string, host string, portText string, unixPath string) {
	trimmed := strings.TrimSpace(listenURI)
	switch {
	case strings.HasPrefix(trimmed, "unix://"):
		return "unix", "127.0.0.1", "60000", strings.TrimPrefix(trimmed, "unix://")
	case strings.HasPrefix(trimmed, "tcp://"):
		remainder := strings.TrimPrefix(trimmed, "tcp://")
		hostPart, portPart, err := net.SplitHostPort(remainder)
		if err != nil {
			return "tcp", "127.0.0.1", "60000", "/tmp/gabriel-greeting-coax.sock"
		}
		if hostPart == "" {
			hostPart = "127.0.0.1"
		}
		return "tcp", hostPart, portPart, "/tmp/gabriel-greeting-coax.sock"
	default:
		return "tcp", "127.0.0.1", "60000", "/tmp/gabriel-greeting-coax.sock"
	}
}

func materializeCompositeListenURI(listenURI string) (string, error) {
	trimmed := strings.TrimSpace(listenURI)
	if !strings.HasPrefix(trimmed, "tcp://") {
		return trimmed, nil
	}
	_, _, portText, _ := compositeListenSnapshot(trimmed)
	if portText != "0" {
		return trimmed, nil
	}
	return nextLoopbackAddress()
}

func compositeKnownAddress(listenURI string) string {
	trimmed := strings.TrimSpace(listenURI)
	switch {
	case strings.HasPrefix(trimmed, "tcp://"):
		_, _, portText, _ := compositeListenSnapshot(trimmed)
		if portText == "0" {
			return ""
		}
		return trimmed
	case strings.HasPrefix(trimmed, "unix://"):
		return trimmed
	default:
		return ""
	}
}

func appBundleInfoValue(plistPath string, key string) (string, error) {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return "", err
	}
	needle := "<key>" + key + "</key>"
	content := string(data)
	index := strings.Index(content, needle)
	if index < 0 {
		return "", fmt.Errorf("%s missing from %s", key, plistPath)
	}
	rest := content[index+len(needle):]
	start := strings.Index(rest, "<string>")
	end := strings.Index(rest, "</string>")
	if start < 0 || end < 0 || end <= start+len("<string>") {
		return "", fmt.Errorf("%s has no string value in %s", key, plistPath)
	}
	return strings.TrimSpace(rest[start+len("<string>") : end]), nil
}

func nextLoopbackAddress() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer listener.Close()
	return "tcp://" + listener.Addr().String(), nil
}

func replaceCommandEnv(env []string, key string, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func connectLaunchedUnix(binaryPath string, origin *sdkdiscover.HolonRef, timeout time.Duration) (activeConnection, error) {
	tempRoot := "/tmp"
	if info, err := os.Stat(tempRoot); err != nil || !info.IsDir() {
		tempRoot = os.TempDir()
	}

	socketDir, err := os.MkdirTemp(tempRoot, "op-unix-")
	if err != nil {
		return activeConnection{}, err
	}
	socketPath := filepath.Join(socketDir, "h.sock")

	conn, err := connectLaunchedAddress(binaryPath, origin, timeout, "unix://"+socketPath, func() error {
		return os.RemoveAll(socketDir)
	})
	if err != nil {
		_ = os.RemoveAll(socketDir)
		return activeConnection{}, err
	}
	return conn, nil
}

func connectLaunchedAddress(
	binaryPath string,
	origin *sdkdiscover.HolonRef,
	timeout time.Duration,
	listenURI string,
	cleanup func() error,
) (activeConnection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.Command(binaryPath, "serve", "--listen", listenURI)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		return activeConnection{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		return activeConnection{}, err
	}

	if err := cmd.Start(); err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		return activeConnection{}, err
	}

	addressCh := make(chan string, 1)
	readErrCh := make(chan error, 2)
	streamReader := func(reader io.Reader, mirror io.Writer) {
		buffered := bufio.NewReader(reader)
		for {
			line, readErr := buffered.ReadString('\n')
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if mirror != nil {
					fmt.Fprintln(mirror, trimmed)
				}
				if address := advertisedListenAddress(trimmed); address != "" {
					select {
					case addressCh <- address:
					default:
					}
					return
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					readErrCh <- readErr
					return
				}
				readErrCh <- io.EOF
				return
			}
		}
	}
	go streamReader(stdout, nil)
	go streamReader(stderr, os.Stderr)

	var (
		address    string
		streamDone int
	)
	for address == "" {
		select {
		case <-ctx.Done():
			_ = stopCommand(cmd)
			if cleanup != nil {
				_ = cleanup()
			}
			return activeConnection{}, fmt.Errorf("server startup timeout")
		case err := <-readErrCh:
			if err != io.EOF {
				_ = stopCommand(cmd)
				if cleanup != nil {
					_ = cleanup()
				}
				return activeConnection{}, err
			}
			streamDone++
			if streamDone == 2 {
				_ = stopCommand(cmd)
				if cleanup != nil {
					_ = cleanup()
				}
				return activeConnection{}, fmt.Errorf("holon did not advertise a listen address")
			}
		case address = <-addressCh:
		}
	}

	conn, err := sdkgrpcclient.Dial(ctx, normalizeDialTarget(address))
	if err != nil {
		_ = stopCommand(cmd)
		return activeConnection{}, err
	}

	originCopy := copyRef(origin)
	if originCopy != nil {
		originCopy.URL = address
	}

	return activeConnection{
		conn: conn,
		disconnect: func() error {
			closeErr := conn.Close()
			killErr := stopCommand(cmd)
			if cleanup != nil {
				_ = cleanup()
			}
			if closeErr != nil {
				return closeErr
			}
			return killErr
		},
		origin: originCopy,
	}, nil
}

func advertisedListenAddress(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "tcp://") || strings.HasPrefix(trimmed, "unix://") {
		return trimmed
	}
	for _, field := range strings.Fields(trimmed) {
		if strings.HasPrefix(field, "tcp://") || strings.HasPrefix(field, "unix://") {
			return strings.TrimSpace(field)
		}
	}
	return ""
}

func autoBuildAndConnect(holonName string, transport string) (activeConnection, error) {
	target, err := holons.ResolveTargetWithOptions(holonName, nil, sdkdiscover.SOURCE, int(connectDispatchTimeout/time.Millisecond))
	if err != nil {
		return activeConnection{}, err
	}
	if target.ManifestErr != nil {
		return activeConnection{}, target.ManifestErr
	}
	if target.Manifest == nil {
		return activeConnection{}, fmt.Errorf("no %s found in %s", identity.ProtoManifestFileName, target.RelativePath)
	}

	pw := progress.New(os.Stderr)
	if !pw.IsTTY() {
		pw = progress.Silence()
	}
	defer pw.Close()

	if _, err := holons.ExecuteLifecycle(holons.OperationBuild, target.Dir, holons.BuildOptions{Progress: pw}); err != nil {
		pw.Keep()
		return activeConnection{}, err
	}
	pw.Clear()

	return connectForRPC(holonName, transport)
}

func cmdGRPCConnected(format Format, uri string, holonName string, args []string, transport string) int {
	calls, noBuild, err := parseConnectedRPCArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op grpc: %v\n", err)
		fmt.Fprintf(os.Stderr, "usage: op %s <method> [--no-build] [json] [<method> [json] ...]\n", uri)
		return 1
	}
	return runConnectedRPC(format, "op grpc", holonName, calls, transport, noBuild)
}

func refLocalPath(ref *sdkdiscover.HolonRef) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("nil holon ref")
	}
	trimmed := strings.TrimSpace(ref.URL)
	if !strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		return trimmed, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	return parsed.Path, nil
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
		if parsed.Host != "" {
			return parsed.Host
		}
	case "unix", "ws", "wss":
		return trimmed
	}
	return trimmed
}

func stopCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "finished") {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func isBuiltBinaryNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), sdkconnect.ErrBinaryNotFound.Error())
}

func isResolveNotFound(message string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(trimmed, "not found")
}

func copyRef(ref *sdkdiscover.HolonRef) *sdkdiscover.HolonRef {
	if ref == nil {
		return nil
	}
	dup := *ref
	if ref.Info != nil {
		info := *ref.Info
		info.Architectures = append([]string(nil), ref.Info.Architectures...)
		info.Identity.Aliases = append([]string(nil), ref.Info.Identity.Aliases...)
		dup.Info = &info
	}
	return &dup
}
