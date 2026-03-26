package main

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
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/go-holons/pkg/transport"
	"google.golang.org/grpc"
)

type mockEchoServer struct {
	sdk     string
	version string
}

func (s mockEchoServer) Ping(_ context.Context, in *PingRequest) (*PingResponse, error) {
	return &PingResponse{
		Message: in.Message,
		SDK:     s.sdk,
		Version: s.version,
	}, nil
}

type mockEchoService interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
}

var mockEchoServiceDesc = grpc.ServiceDesc{
	ServiceName: "echo.v1.Echo",
	HandlerType: (*mockEchoService)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler: func(
				srv interface{},
				ctx context.Context,
				dec func(interface{}) error,
				interceptor grpc.UnaryServerInterceptor,
			) (interface{}, error) {
				in := new(PingRequest)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return srv.(mockEchoService).Ping(ctx, in)
				}
				info := &grpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: "/echo.v1.Echo/Ping",
				}
				handler := func(ctx context.Context, req interface{}) (interface{}, error) {
					return srv.(mockEchoService).Ping(ctx, req.(*PingRequest))
				}
				return interceptor(ctx, in, info, handler)
			},
		},
	},
}

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_ECHO_CLIENT_STDIO_SERVER") == "1" && len(os.Args) > 1 && os.Args[1] == "serve" {
		os.Exit(runStdioServerHelper(os.Args))
	}

	if os.Getenv("GO_WANT_ECHO_CLIENT_HELPER") == "1" {
		os.Args = append([]string{os.Args[0]}, helperArgs(os.Args)...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestEchoClient_TCP_RoundTrip(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer lis.Close()

	stop := startMockEchoGRPCServer(t, lis, mockEchoServer{sdk: "go-holons", version: "0.3.0"})
	defer stop()

	uri := "tcp://" + lis.Addr().String()
	stdout, stderr, err := runEchoClientHelper(t, "--message", "hello-tcp", "--server-sdk", "go-holons", uri)
	if err != nil {
		t.Fatalf("echo-client failed: %v\nstderr:\n%s", err, stderr)
	}

	out := parseClientOutput(t, stdout)
	if out["status"] != "pass" {
		t.Fatalf("status = %v, want pass", out["status"])
	}
	if out["response_sdk"] != "go-holons" {
		t.Fatalf("response_sdk = %v, want go-holons", out["response_sdk"])
	}
}

func TestEchoClient_Unix_RoundTrip(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("echo-client-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(socketPath) })

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer lis.Close()

	stop := startMockEchoGRPCServer(t, lis, mockEchoServer{sdk: "go-holons", version: "0.3.0"})
	defer stop()

	uri := "unix://" + socketPath
	stdout, stderr, err := runEchoClientHelper(t, "--message", "hello-unix", uri)
	if err != nil {
		t.Fatalf("echo-client failed: %v\nstderr:\n%s", err, stderr)
	}
	out := parseClientOutput(t, stdout)
	if out["status"] != "pass" {
		t.Fatalf("status = %v, want pass", out["status"])
	}
}

func TestEchoClient_Stdio_RoundTrip(t *testing.T) {
	binaryPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	stdout, stderr, err := runEchoClientHelperWithEnv(
		t,
		[]string{"GO_WANT_ECHO_CLIENT_STDIO_SERVER=1"},
		"--message",
		"hello-stdio",
		"--server-sdk",
		"go-holons",
		"--stdio-bin",
		binaryPath,
		"stdio://",
	)
	if err != nil {
		t.Fatalf("echo-client stdio failed: %v\nstderr:\n%s", err, stderr)
	}

	out := parseClientOutput(t, stdout)
	if out["status"] != "pass" {
		t.Fatalf("status = %v, want pass", out["status"])
	}
	if out["response_sdk"] != "go-holons" {
		t.Fatalf("response_sdk = %v, want go-holons", out["response_sdk"])
	}
}

func TestEchoClient_UnsupportedURI(t *testing.T) {
	_, stderr, err := runEchoClientHelper(t, "bad://example")
	if err == nil {
		t.Fatal("expected failure for unsupported URI")
	}
	if !strings.Contains(stderr, "unsupported URI") {
		t.Fatalf("stderr = %q, want unsupported URI message", stderr)
	}
}

func TestNormalizeTarget(t *testing.T) {
	target, dialer, err := normalizeTarget("tcp://127.0.0.1:9090")
	if err != nil {
		t.Fatalf("normalize tcp: %v", err)
	}
	if target != "127.0.0.1:9090" || dialer != nil {
		t.Fatalf("unexpected tcp normalize result: target=%q dialer=%v", target, dialer != nil)
	}

	target, dialer, err = normalizeTarget("unix:///tmp/echo.sock")
	if err != nil {
		t.Fatalf("normalize unix: %v", err)
	}
	if target != "passthrough:///unix" || dialer == nil {
		t.Fatalf("unexpected unix normalize result: target=%q dialer=%v", target, dialer != nil)
	}

	if got := normalizeURI("stdio"); got != "stdio://" {
		t.Fatalf("normalizeURI(stdio) = %q, want stdio://", got)
	}
	if !isStdioURI("stdio://") || !isStdioURI("stdio") {
		t.Fatal("expected stdio URI detection to include stdio:// and stdio")
	}

	if _, _, err := normalizeTarget("stdio://"); err == nil {
		t.Fatal("expected unsupported stdio URI")
	}
}

func TestDefaultGoBinary(t *testing.T) {
	t.Setenv("GO_BIN", "go-from-env")
	if got := defaultGoBinary(); got != "go-from-env" {
		t.Fatalf("defaultGoBinary with env = %q, want go-from-env", got)
	}

	t.Setenv("GO_BIN", "")
	if got := defaultGoBinary(); got != "go" {
		t.Fatalf("defaultGoBinary fallback = %q, want go", got)
	}
}

func TestDial_StdioGoRunStartFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, _, err := dial(ctx, "stdio://", "go-holons", filepath.Join(t.TempDir(), "missing-go"), "")
	if err == nil {
		t.Fatal("expected dial stdio go-run failure")
	}
	if !strings.Contains(err.Error(), "start stdio server") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPipeConnHelpers(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	p := &pipeConn{
		reader: bytes.NewReader(nil),
		writer: writeNopCloser{Writer: buf},
	}

	if err := p.SetDeadline(time.Now()); err != nil {
		t.Fatalf("SetDeadline returned error: %v", err)
	}
	if err := p.SetReadDeadline(time.Now()); err != nil {
		t.Fatalf("SetReadDeadline returned error: %v", err)
	}
	if err := p.SetWriteDeadline(time.Now()); err != nil {
		t.Fatalf("SetWriteDeadline returned error: %v", err)
	}

	if p.LocalAddr().Network() != "pipe" || p.LocalAddr().String() != "stdio://" {
		t.Fatalf("unexpected local addr: %s %s", p.LocalAddr().Network(), p.LocalAddr().String())
	}
	if p.RemoteAddr().Network() != "pipe" || p.RemoteAddr().String() != "stdio://" {
		t.Fatalf("unexpected remote addr: %s %s", p.RemoteAddr().Network(), p.RemoteAddr().String())
	}

	if _, err := p.Write([]byte("x")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if got := buf.String(); got != "x" {
		t.Fatalf("buffer = %q, want x", got)
	}
}

func startMockEchoGRPCServer(t *testing.T, lis net.Listener, svc mockEchoServer) func() {
	t.Helper()

	srv := grpc.NewServer(grpc.ForceServerCodec(jsonCodec{}))
	srv.RegisterService(&mockEchoServiceDesc, svc)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(lis)
	}()

	return func() {
		srv.GracefulStop()
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) && !strings.Contains(err.Error(), "closed network connection") {
				t.Fatalf("mock echo server exit error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for mock echo server shutdown")
		}
	}
}

func runEchoClientHelper(t *testing.T, args ...string) (string, string, error) {
	return runEchoClientHelperWithEnv(t, nil, args...)
}

func runEchoClientHelperWithEnv(t *testing.T, extraEnv []string, args ...string) (string, string, error) {
	t.Helper()

	cmdArgs := []string{"-test.run=TestEchoClient_TCP_RoundTrip"}
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_ECHO_CLIENT_HELPER=1")
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func parseClientOutput(t *testing.T, output string) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("parse client output: %v\noutput:\n%s", err, output)
	}
	return out
}

func helperArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1:]
			}
			return nil
		}
	}
	return nil
}

func runStdioServerHelper(args []string) int {
	sdk := helperArgValue(args, "--sdk", "go-holons")
	version := helperArgValue(args, "--version", "0.3.0")

	lis, err := transport.Listen("stdio://")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer lis.Close()

	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsonCodec{}))
	grpcServer.RegisterService(&mockEchoServiceDesc, mockEchoServer{
		sdk:     sdk,
		version: version,
	})

	if err := grpcServer.Serve(lis); err != nil && !isBenignStdioServeError(err) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func helperArgValue(args []string, key string, fallback string) string {
	for i := 1; i < len(args); i++ {
		if args[i] != key {
			continue
		}
		if i+1 < len(args) {
			return args[i+1]
		}
	}
	return fallback
}

func isBenignStdioServeError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, grpc.ErrServerStopped) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "eof") || strings.Contains(msg, "closed network connection")
}

type writeNopCloser struct {
	io.Writer
}

func (writeNopCloser) Close() error { return nil }
