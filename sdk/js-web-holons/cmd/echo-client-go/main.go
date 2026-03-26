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
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultSDK       = "js-web-holons"
	defaultServerSDK = "go-holons"
	defaultURI       = "stdio://"
	defaultMessage   = "hello"
	defaultTimeoutMS = 5000
)

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
	args, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	timeout := time.Duration(args.timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultTimeoutMS * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startedAt := time.Now()

	conn, child, err := dial(ctx, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	defer stopChild(child)

	var out PingResponse
	if err := conn.Invoke(ctx, "/echo.v1.Echo/Ping", &PingRequest{Message: args.message}, &out); err != nil {
		fmt.Fprintf(os.Stderr, "invoke failed: %v\n", err)
		os.Exit(1)
	}
	if out.Message != args.message {
		fmt.Fprintf(os.Stderr, "unexpected echo message: %q\n", out.Message)
		os.Exit(1)
	}

	result := map[string]interface{}{
		"status":       "pass",
		"sdk":          args.sdk,
		"server_sdk":   args.serverSDK,
		"latency_ms":   time.Since(startedAt).Milliseconds(),
		"response_sdk": out.SDK,
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode failed: %v\n", err)
		os.Exit(1)
	}
}

type options struct {
	uri       string
	sdk       string
	serverSDK string
	message   string
	timeoutMS int
	goBinary  string
}

func parseFlags() (options, error) {
	sdk := flag.String("sdk", defaultSDK, "sdk name")
	serverSDK := flag.String("server-sdk", defaultServerSDK, "expected remote sdk name")
	message := flag.String("message", defaultMessage, "Ping request message")
	timeoutMS := flag.Int("timeout-ms", defaultTimeoutMS, "dial+invoke timeout in milliseconds")
	goBinary := flag.String("go", defaultGoBinary(), "go binary used to spawn stdio server")
	flag.Parse()

	uri := defaultURI
	switch flag.NArg() {
	case 0:
	case 1:
		uri = normalizeURI(flag.Arg(0))
	default:
		return options{}, fmt.Errorf("usage: echo-client-go [flags] [tcp://host:port|unix://path|stdio://]")
	}

	return options{
		uri:       uri,
		sdk:       *sdk,
		serverSDK: *serverSDK,
		message:   *message,
		timeoutMS: *timeoutMS,
		goBinary:  *goBinary,
	}, nil
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

func dial(ctx context.Context, args options) (*grpc.ClientConn, *exec.Cmd, error) {
	if args.uri == "stdio://" {
		return dialStdio(ctx, args.goBinary, args.serverSDK)
	}

	target, dialer, err := normalizeTarget(args.uri)
	if err != nil {
		return nil, nil, err
	}

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	}
	if dialer != nil {
		dialOptions = append(dialOptions, grpc.WithContextDialer(dialer))
	}

	//nolint:staticcheck // DialContext is required with custom dialers + blocking connect.
	conn, err := grpc.DialContext(ctx, target, dialOptions...)
	if err != nil {
		return nil, nil, err
	}
	return conn, nil, nil
}

func normalizeTarget(uri string) (string, func(context.Context, string) (net.Conn, error), error) {
	if !strings.Contains(uri, "://") {
		return uri, nil, nil
	}

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

func dialStdio(ctx context.Context, goBinary, serverSDK string) (*grpc.ClientConn, *exec.Cmd, error) {
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
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, nil, fmt.Errorf("stdio server startup failed: %w", readErr)
		}
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
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

	//nolint:staticcheck // DialContext is required for custom dialers + blocking connect.
	conn, err := grpc.DialContext(
		ctx,
		"passthrough:///stdio",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, nil, fmt.Errorf("grpc handshake over stdio: %w", err)
	}

	return conn, cmd, nil
}

func stopChild(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
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
