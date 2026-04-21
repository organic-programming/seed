package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/transport"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultSDK       = "cpp-holons"
	defaultServerSDK = "go-holons"
	defaultURI       = "stdio://"
	defaultMessage   = "hello"
	defaultTimeoutMS = 5000
	defaultVersion   = "0.1.0"
)

type PingRequest struct {
	Message string `json:"message"`
}

type PingResponse struct {
	Message string `json:"message"`
	SDK     string `json:"sdk"`
	Version string `json:"version"`
}

type options struct {
	uri       string
	sdk       string
	serverSDK string
	message   string
	timeoutMS int
	goBinary  string
}

type echoService interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
}

type echoServer struct {
	sdk     string
	version string
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (s echoServer) Ping(_ context.Context, in *PingRequest) (*PingResponse, error) {
	return &PingResponse{
		Message: in.Message,
		SDK:     s.sdk,
		Version: s.version,
	}, nil
}

var echoServiceDesc = grpc.ServiceDesc{
	ServiceName: "echo.v1.Echo",
	HandlerType: (*echoService)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler: func(
				srv any,
				ctx context.Context,
				dec func(any) error,
				interceptor grpc.UnaryServerInterceptor,
			) (any, error) {
				in := new(PingRequest)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return srv.(echoService).Ping(ctx, in)
				}
				info := &grpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: "/echo.v1.Echo/Ping",
				}
				handler := func(ctx context.Context, req any) (any, error) {
					return srv.(echoService).Ping(ctx, req.(*PingRequest))
				}
				return interceptor(ctx, in, info, handler)
			},
		},
	},
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

	conn, child, cleanup, err := dial(ctx, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = conn.Close()
	}()
	defer stopChild(child)
	if cleanup != nil {
		defer cleanup()
	}

	var out PingResponse
	if err := conn.Invoke(
		ctx,
		"/echo.v1.Echo/Ping",
		&PingRequest{Message: args.message},
		&out,
		grpc.ForceCodec(jsonCodec{}),
	); err != nil {
		fmt.Fprintf(os.Stderr, "invoke failed: %v\n", err)
		os.Exit(1)
	}
	if out.Message != args.message {
		fmt.Fprintf(os.Stderr, "unexpected echo message: %q\n", out.Message)
		os.Exit(1)
	}

	result := map[string]any{
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
		return options{}, fmt.Errorf(
			"usage: echo-client-go [flags] [tcp://host:port|unix://path|stdio://|ws://host:port/grpc]",
		)
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

func dial(
	ctx context.Context,
	args options,
) (*grpc.ClientConn, *exec.Cmd, func(), error) {
	if isStdioURI(args.uri) {
		conn, child, err := dialStdio(ctx, args.goBinary, args.serverSDK)
		return conn, child, nil, err
	}

	if strings.HasPrefix(args.uri, "ws://") || strings.HasPrefix(args.uri, "wss://") {
		wsURI, err := normalizeWebSocketURI(args.uri)
		if err != nil {
			return nil, nil, nil, err
		}
		conn, cleanup, err := dialWebSocket(ctx, wsURI, args.serverSDK)
		return conn, nil, cleanup, err
	}

	target, dialer, err := normalizeTarget(args.uri)
	if err != nil {
		return nil, nil, nil, err
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
		return nil, nil, nil, err
	}
	return conn, nil, nil, nil
}

func normalizeWebSocketURI(rawURI string) (string, error) {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("invalid websocket URI: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return "", fmt.Errorf("unsupported websocket URI: %s", rawURI)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("websocket URI missing host: %s", rawURI)
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/grpc"
	}
	return parsed.String(), nil
}

func dialWebSocketWithRetry(ctx context.Context, wsURI string) (*grpc.ClientConn, error) {
	var lastErr error
	deadline := time.Now().Add(3 * time.Second)

	for {
		conn, err := grpcclient.DialWebSocket(ctx, wsURI)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		if time.Now().After(deadline) {
			return nil, lastErr
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func dialWebSocket(ctx context.Context, wsURI, serverSDK string) (*grpc.ClientConn, func(), error) {
	parsed, err := url.Parse(wsURI)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid websocket URI: %w", err)
	}

	// A ws://...:0 URI cannot be dialed directly. Treat it as a request for an
	// in-process websocket round-trip and bind an ephemeral local WS listener.
	if parsed.Port() == "0" {
		return dialEphemeralWebSocket(ctx, serverSDK)
	}

	conn, err := dialWebSocketWithRetry(ctx, wsURI)
	return conn, nil, err
}

func dialEphemeralWebSocket(ctx context.Context, serverSDK string) (*grpc.ClientConn, func(), error) {
	lis, err := transport.Listen("ws://127.0.0.1:0/grpc")
	if err != nil {
		return nil, nil, err
	}

	server := grpc.NewServer(grpc.ForceServerCodec(jsonCodec{}))
	server.RegisterService(&echoServiceDesc, echoServer{
		sdk:     serverSDK,
		version: defaultVersion,
	})

	done := make(chan error, 1)
	go func() {
		done <- server.Serve(lis)
	}()

	conn, err := dialWebSocketWithRetry(ctx, lis.Addr().String())
	if err != nil {
		server.Stop()
		_ = lis.Close()
		return nil, nil, err
	}

	cleanup := func() {
		server.Stop()
		_ = lis.Close()
		select {
		case <-done:
		case <-time.After(300 * time.Millisecond):
		}
	}

	return conn, cleanup, nil
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

func isStdioURI(uri string) bool {
	return uri == "stdio://" || uri == "stdio"
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
