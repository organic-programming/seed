package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	sdkgrpcclient "github.com/organic-programming/go-holons/pkg/grpcclient"
	openv "github.com/organic-programming/grace-op/internal/env"
	internalgrpc "github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/progress"
	"google.golang.org/grpc"
)

const connectDispatchTimeout = 10 * time.Second

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

func runConnectedRPC(
	format Format,
	errPrefix string,
	holonName string,
	method string,
	inputJSON string,
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

	result, err := internalgrpc.InvokeConn(ctx, conn.conn, method, inputJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", errPrefix, err)
		return 1
	}

	fmt.Println(formatRPCOutput(format, method, []byte(result.Output)))
	return 0
}

func parseConnectedRPCArgs(args []string) (method string, inputJSON string, noBuild bool, err error) {
	if len(args) < 1 {
		return "", "", false, fmt.Errorf("method required")
	}

	method = args[0]
	remaining := args[1:]
	inputJSON = "{}"

	if len(remaining) > 0 && remaining[0] == "--no-build" {
		noBuild = true
		remaining = remaining[1:]
	}

	if len(remaining) > 0 {
		inputJSON = remaining[0]
	}

	for _, arg := range remaining[1:] {
		if strings.TrimSpace(arg) == "--no-build" {
			return "", "", false, fmt.Errorf("--no-build must come immediately after the method")
		}
	}

	return method, inputJSON, noBuild, nil
}

func connectForRPC(holonName string, transport string) (activeConnection, error) {
	return connectForRPCWithTimeout(holonName, transport, connectDispatchTimeout)
}

func connectForRPCWithTimeout(holonName string, transport string, timeout time.Duration) (activeConnection, error) {
	root := openv.Root()
	specifiers := sdkdiscover.ALL
	timeoutMS := int(timeout / time.Millisecond)

	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "", "auto":
		result := holons.ConnectRef(holonName, &root, specifiers, timeoutMS)
		if result.Error != "" {
			return activeConnection{}, errors.New(result.Error)
		}
		return activeConnection{
			conn:       result.Channel,
			disconnect: func() error { return sdkconnect.Disconnect(result) },
			origin:     result.Origin,
		}, nil
	case "stdio":
		return connectForcedTransport(holonName, "stdio", &root, specifiers, timeout)
	case "tcp":
		return connectForcedTransport(holonName, "tcp", &root, specifiers, timeout)
	default:
		result := holons.ConnectRef(holonName, &root, specifiers, timeoutMS)
		if result.Error != "" {
			return activeConnection{}, errors.New(result.Error)
		}
		return activeConnection{
			conn:       result.Channel,
			disconnect: func() error { return sdkconnect.Disconnect(result) },
			origin:     result.Origin,
		}, nil
	}
}

func connectForcedTransport(holonName string, transport string, root *string, specifiers int, timeout time.Duration) (activeConnection, error) {
	resolved := holons.ResolveRef(holonName, root, specifiers, int(timeout/time.Millisecond))
	if resolved.Error != "" {
		return activeConnection{}, errors.New(resolved.Error)
	}
	if resolved.Ref == nil {
		return activeConnection{}, fmt.Errorf("holon %q not found", holonName)
	}

	binaryPath, err := binaryPathForRef(resolved.Ref, holonName)
	if err != nil {
		return activeConnection{}, err
	}

	switch transport {
	case "stdio":
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
			origin: resolved.Ref,
		}, nil
	case "tcp":
		return connectLaunchedTCP(binaryPath, resolved.Ref, timeout)
	default:
		return activeConnection{}, fmt.Errorf("unsupported forced transport %q", transport)
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.Command(binaryPath, "serve", "--listen", "tcp://127.0.0.1:0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return activeConnection{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return activeConnection{}, err
	}

	if err := cmd.Start(); err != nil {
		return activeConnection{}, err
	}

	go func() {
		_, _ = io.Copy(os.Stderr, stderr)
	}()

	addressCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		for {
			line, readErr := reader.ReadString('\n')
			if strings.TrimSpace(line) != "" {
				addressCh <- strings.TrimSpace(line)
				return
			}
			if readErr != nil {
				if readErr == io.EOF {
					errCh <- fmt.Errorf("holon did not advertise a listen address")
				} else {
					errCh <- readErr
				}
				return
			}
		}
	}()

	var address string
	select {
	case <-ctx.Done():
		_ = stopCommand(cmd)
		return activeConnection{}, fmt.Errorf("server startup timeout")
	case err := <-errCh:
		_ = stopCommand(cmd)
		return activeConnection{}, err
	case address = <-addressCh:
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
			if closeErr != nil {
				return closeErr
			}
			return killErr
		},
		origin: originCopy,
	}, nil
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
	method, inputJSON, noBuild, err := parseConnectedRPCArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op grpc: %v\n", err)
		fmt.Fprintf(os.Stderr, "usage: op %s <method> [--no-build] [json]\n", uri)
		return 1
	}
	return runConnectedRPC(format, "op grpc", holonName, method, inputJSON, transport, noBuild)
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
