package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultDialTimeout = 5 * time.Second

type PingRequest struct {
	Message string `json:"message"`
}

type PingResponse struct {
	Message string `json:"message"`
	SDK     string `json:"sdk"`
	Version string `json:"version"`
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func main() {
	sdk := flag.String("sdk", "go-holons", "sdk name")
	serverSDK := flag.String("server-sdk", "unknown", "expected remote sdk name")
	message := flag.String("message", "hello", "Ping request message")
	timeoutMs := flag.Int("timeout-ms", int(defaultDialTimeout/time.Millisecond), "dial+invoke timeout in milliseconds")
	goBinary := flag.String("go", defaultGoBinary(), "go binary used to spawn stdio echo server")
	stdioBin := flag.String("stdio-bin", strings.TrimSpace(os.Getenv("HOLONS_ECHO_SERVER_BIN")), "echo-server binary path used for stdio://")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/echo-client [--sdk name] [--server-sdk name] [--message hello] [--go go] [--stdio-bin /path/to/echo-server] [tcp://host:port|unix://path|stdio://]")
		os.Exit(2)
	}

	uri := normalizeURI(flag.Arg(0))

	timeout := time.Duration(*timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultDialTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, child, err := dial(ctx, uri, *serverSDK, strings.TrimSpace(*goBinary), strings.TrimSpace(*stdioBin))
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	defer stopChild(child)

	started := time.Now()
	var out PingResponse
	err = conn.Invoke(ctx, "/echo.v1.Echo/Ping", &PingRequest{Message: *message}, &out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invoke failed: %v\n", err)
		os.Exit(1)
	}
	if out.Message != *message {
		fmt.Fprintf(os.Stderr, "unexpected echo message: %q\n", out.Message)
		os.Exit(1)
	}

	result := map[string]interface{}{
		"status":       "pass",
		"sdk":          *sdk,
		"server_sdk":   *serverSDK,
		"latency_ms":   time.Since(started).Milliseconds(),
		"response_sdk": out.SDK,
	}

	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(result)
}

func defaultGoBinary() string {
	if fromEnv := strings.TrimSpace(os.Getenv("GO_BIN")); fromEnv != "" {
		return fromEnv
	}
	return "go"
}

func normalizeURI(uri string) string {
	if uri == "stdio" {
		return "stdio://"
	}
	return uri
}

func dial(
	ctx context.Context,
	uri string,
	serverSDK string,
	goBinary string,
	stdioBin string,
) (*grpc.ClientConn, *exec.Cmd, error) {
	if isStdioURI(uri) {
		if stdioBin != "" {
			return dialStdioBinary(ctx, stdioBin, serverSDK)
		}
		return dialStdioGoRun(ctx, goBinary, serverSDK)
	}

	target, dialer, err := normalizeTarget(uri)
	if err != nil {
		return nil, nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	}
	if dialer != nil {
		opts = append(opts, grpc.WithContextDialer(dialer))
	}

	//nolint:staticcheck // DialContext is required with custom dialers + blocking connect.
	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, nil, err
	}
	return conn, nil, nil
}

func dialStdioBinary(ctx context.Context, binaryPath string, serverSDK string) (*grpc.ClientConn, *exec.Cmd, error) {
	cmd := exec.CommandContext(
		ctx,
		binaryPath,
		"serve",
		"--listen",
		"stdio://",
		"--sdk",
		serverSDK,
	)
	return dialStdioCommand(ctx, cmd)
}

func dialStdioGoRun(ctx context.Context, goBinary string, serverSDK string) (*grpc.ClientConn, *exec.Cmd, error) {
	if goBinary == "" {
		goBinary = "go"
	}
	cmd := exec.CommandContext(
		ctx,
		goBinary,
		"run",
		"./cmd/echo-server",
		"--listen",
		"stdio://",
		"--sdk",
		serverSDK,
	)
	return dialStdioCommand(ctx, cmd)
}

func dialStdioCommand(ctx context.Context, cmd *exec.Cmd) (*grpc.ClientConn, *exec.Cmd, error) {
	cmd.Stderr = os.Stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start stdio server: %w", err)
	}

	firstByte := make([]byte, 1)
	readCh := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(stdoutPipe, firstByte)
		readCh <- readErr
	}()

	select {
	case readErr := <-readCh:
		if readErr != nil {
			killAndWait(cmd)
			return nil, nil, fmt.Errorf("stdio server startup failed: %w", readErr)
		}
	case <-ctx.Done():
		killAndWait(cmd)
		return nil, nil, fmt.Errorf("stdio server startup timeout")
	}

	pConn := &pipeConn{
		reader: io.MultiReader(bytes.NewReader(firstByte), stdoutPipe),
		writer: stdinPipe,
	}

	var dialOnce sync.Once
	dialer := func(context.Context, string) (net.Conn, error) {
		var conn net.Conn
		dialOnce.Do(func() {
			conn = pConn
		})
		if conn == nil {
			return nil, fmt.Errorf("stdio pipe already consumed")
		}
		return conn, nil
	}

	//nolint:staticcheck // DialContext is required with custom dialers + blocking connect.
	conn, err := grpc.DialContext(
		ctx,
		"passthrough:///stdio",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		killAndWait(cmd)
		return nil, nil, fmt.Errorf("grpc handshake over stdio: %w", err)
	}

	return conn, cmd, nil
}

func stopChild(cmd *exec.Cmd) {
	killAndWait(cmd)
}

func killAndWait(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

func isStdioURI(uri string) bool {
	return uri == "stdio://" || uri == "stdio"
}

func normalizeTarget(uri string) (string, func(context.Context, string) (net.Conn, error), error) {
	if strings.HasPrefix(uri, "tcp://") {
		return strings.TrimPrefix(uri, "tcp://"), nil, nil
	}
	if strings.HasPrefix(uri, "unix://") {
		path := strings.TrimPrefix(uri, "unix://")
		dialer := func(_ context.Context, _ string) (net.Conn, error) {
			return net.DialTimeout("unix", path, 5*time.Second)
		}
		return "passthrough:///unix", dialer, nil
	}

	return "", nil, fmt.Errorf("unsupported URI: %s", uri)
}

type pipeConn struct {
	reader io.Reader
	writer io.WriteCloser
}

func (c *pipeConn) Read(p []byte) (int, error)         { return c.reader.Read(p) }
func (c *pipeConn) Write(p []byte) (int, error)        { return c.writer.Write(p) }
func (c *pipeConn) Close() error                       { return c.writer.Close() }
func (c *pipeConn) LocalAddr() net.Addr                { return pipeAddr{} }
func (c *pipeConn) RemoteAddr() net.Addr               { return pipeAddr{} }
func (c *pipeConn) SetDeadline(_ time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(_ time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "stdio://" }
