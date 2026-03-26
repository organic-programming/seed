package grpcclient_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/serve"
	"github.com/organic-programming/go-holons/pkg/transport"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	testgrpc "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/status"
)

type echoTestServer struct {
	testgrpc.UnimplementedTestServiceServer
}

func (s *echoTestServer) EmptyCall(context.Context, *testgrpc.Empty) (*testgrpc.Empty, error) {
	return &testgrpc.Empty{}, nil
}

func (s *echoTestServer) UnaryCall(ctx context.Context, in *testgrpc.SimpleRequest) (*testgrpc.SimpleResponse, error) {
	payload := in.GetPayload()
	if payload == nil {
		payload = &testgrpc.Payload{Type: testgrpc.PayloadType_COMPRESSABLE, Body: []byte("echo")}
	}

	if delay := parseSleepMillis(payload.GetBody()); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return &testgrpc.SimpleResponse{
		Payload: &testgrpc.Payload{
			Type: payload.GetType(),
			Body: append([]byte(nil), payload.GetBody()...),
		},
	}, nil
}

func (s *echoTestServer) FullDuplexCall(stream testgrpc.TestService_FullDuplexCallServer) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		payload := req.GetPayload()
		if payload == nil {
			payload = &testgrpc.Payload{Type: testgrpc.PayloadType_COMPRESSABLE, Body: []byte{}}
		}
		if err := stream.Send(&testgrpc.StreamingOutputCallResponse{
			Payload: &testgrpc.Payload{
				Type: payload.GetType(),
				Body: append([]byte(nil), payload.GetBody()...),
			},
		}); err != nil {
			return err
		}
	}
}

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		listenURI := serve.ParseFlags(os.Args[2:])
		describe.UseStaticResponse(grpcclientStaticDescribeResponse())
		err := serve.RunWithOptions(listenURI, func(s *grpc.Server) {
			testgrpc.RegisterTestServiceServer(s, &echoTestServer{})
		}, false)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func grpcclientStaticDescribeResponse() *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Schema:     "holon/v1",
				Uuid:       "grpcclient-test-0000",
				GivenName:  "GRPCClient",
				FamilyName: "Helper",
				Motto:      "Provides static Describe data for stdio helper tests.",
				Composer:   "go-holons",
				Status:     "draft",
				Born:       "2026-03-23",
			},
			Lang: "go",
		},
		Services: []*holonsv1.ServiceDoc{{
			Name:        "grpc.testing.TestService",
			Description: "Interop service used by grpcclient tests.",
			Methods: []*holonsv1.MethodDoc{{
				Name:        "UnaryCall",
				Description: "Echoes the inbound payload.",
				InputType:   "grpc.testing.SimpleRequest",
				OutputType:  "grpc.testing.SimpleResponse",
			}},
		}},
	}
}

func TestDialTCPRoundTrip(t *testing.T) {
	lis, err := transport.Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	startEchoGRPCServer(t, lis)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpcclient.Dial(ctx, lis.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	requireUnaryEchoEventually(t, conn, "tcp-echo")
	requireStreamEcho(t, conn, []string{"tcp-s1", "tcp-s2"})
}

func TestDialUnixRoundTrip(t *testing.T) {
	sockPath := t.TempDir() + "/grpc.sock"
	lis, err := transport.Listen("unix://" + sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	startEchoGRPCServer(t, lis)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpcclient.Dial(ctx, "unix://"+sockPath)
	if err != nil {
		t.Fatalf("dial unix: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	requireUnaryEchoEventually(t, conn, "unix-echo")
	requireStreamEcho(t, conn, []string{"unix-s1", "unix-s2"})
}

func TestDialWebSocketRoundTrip(t *testing.T) {
	lis, err := transport.Listen("ws://127.0.0.1:0/grpc")
	if err != nil {
		t.Fatalf("listen ws: %v", err)
	}
	startEchoGRPCServer(t, lis)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var conn *grpc.ClientConn
	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err = grpcclient.DialWebSocket(ctx, lis.Addr().String())
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial ws failed after retries: %v", err)
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Cleanup(func() { _ = conn.Close() })

	requireUnaryEchoEventually(t, conn, "ws-echo")
	requireStreamEcho(t, conn, []string{"ws-s1", "ws-s2"})
}

func TestDialSecureWebSocketRoundTrip(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)
	listenURI := fmt.Sprintf(
		"wss://127.0.0.1:0/grpc?cert=%s&key=%s",
		url.QueryEscape(certFile),
		url.QueryEscape(keyFile),
	)

	lis, err := transport.Listen(listenURI)
	if err != nil {
		t.Fatalf("listen wss: %v", err)
	}
	startEchoGRPCServer(t, lis)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	conn, err := grpcclient.DialWebSocketWithOptions(ctx, lis.Addr().String(), grpcclient.WebSocketDialOptions{
		HTTPClient: httpClient,
	})
	if err != nil {
		t.Fatalf("dial wss: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	requireUnaryEchoEventually(t, conn, "wss-echo")
	requireStreamEcho(t, conn, []string{"wss-s1", "wss-s2"})
}

func TestDialErrorCases(t *testing.T) {
	testCases := []struct {
		name    string
		address string
	}{
		{name: "invalid-unix-uri", address: "unix://"},
		{name: "unreachable-host", address: "127.0.0.1:1"},
		{name: "bad-scheme", address: "bad://host:12345"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			conn, err := grpcclient.Dial(ctx, tc.address)
			if err != nil {
				return
			}
			defer conn.Close()

			rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer rpcCancel()

			_, err = unaryEcho(rpcCtx, conn, "should-fail")
			if err == nil {
				t.Fatalf("expected RPC failure for %q", tc.address)
			}
		})
	}
}

func TestDialWebSocketErrorCases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := grpcclient.DialWebSocket(ctx, "http://127.0.0.1:1234/grpc"); err == nil {
		t.Fatal("expected error for invalid websocket URI scheme")
	}

	if _, err := grpcclient.DialWebSocket(ctx, "ws://127.0.0.1:1/grpc"); err == nil {
		t.Fatal("expected dial failure for unreachable websocket server")
	}
}

func TestDialStdioIntegration(t *testing.T) {
	binaryPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	conn, cmd, err := grpcclient.DialStdio(ctx, binaryPath)
	if err != nil {
		t.Fatalf("DialStdio: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	requireUnaryEchoEventually(t, conn, "stdio-echo")
	requireStreamEcho(t, conn, []string{"stdio-s1", "stdio-s2"})
}

func TestDialUnary_ContextCancellation(t *testing.T) {
	lis, err := transport.Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	startEchoGRPCServer(t, lis)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpcclient.Dial(ctx, lis.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	callCtx, callCancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, invokeErr := unaryEcho(callCtx, conn, "sleep-ms:1500")
		errCh <- invokeErr
	}()

	time.Sleep(100 * time.Millisecond)
	callCancel()

	select {
	case invokeErr := <-errCh:
		if invokeErr == nil {
			t.Fatal("expected cancellation error")
		}
		if code := status.Code(invokeErr); code != codes.Canceled && !errors.Is(invokeErr, context.Canceled) {
			t.Fatalf("expected canceled status, got %v (%v)", code, invokeErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for canceled RPC")
	}
}

func TestDialUnary_DeadlinePropagation(t *testing.T) {
	lis, err := transport.Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	startEchoGRPCServer(t, lis)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpcclient.Dial(ctx, lis.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	callCtx, callCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer callCancel()
	start := time.Now()
	_, invokeErr := unaryEcho(callCtx, conn, "sleep-ms:1500")
	if invokeErr == nil {
		t.Fatal("expected deadline exceeded error")
	}
	if code := status.Code(invokeErr); code != codes.DeadlineExceeded && !errors.Is(invokeErr, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded status, got %v (%v)", code, invokeErr)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("deadline propagation too slow: %v", elapsed)
	}
}

func TestDial_ReconnectAfterServerRestart(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	addr := lis.Addr().String()

	srv1 := grpc.NewServer()
	testgrpc.RegisterTestServiceServer(srv1, &echoTestServer{})
	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- srv1.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpcclient.Dial(ctx, addr)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	requireUnaryEchoEventually(t, conn, "before-restart")

	srv1.Stop()
	select {
	case <-errCh1:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first server shutdown")
	}

	lis2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("relisten tcp: %v", err)
	}
	defer lis2.Close()

	srv2 := grpc.NewServer()
	testgrpc.RegisterTestServiceServer(srv2, &echoTestServer{})
	errCh2 := make(chan error, 1)
	go func() {
		errCh2 <- srv2.Serve(lis2)
	}()
	defer func() {
		srv2.Stop()
		select {
		case <-errCh2:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for second server shutdown")
		}
	}()

	requireUnaryEchoEventually(t, conn, "after-restart")
}

func TestDial_NonExistentServerReturnsUnavailable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpcclient.Dial(ctx, "127.0.0.1:1")
	if err != nil {
		t.Fatalf("dial unexpectedly failed: %v", err)
	}
	defer conn.Close()

	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer rpcCancel()

	_, err = unaryEcho(rpcCtx, conn, "hello")
	if err == nil {
		t.Fatal("expected unavailable RPC error")
	}
	code := status.Code(err)
	if code != codes.Unavailable && code != codes.DeadlineExceeded {
		t.Fatalf("unexpected code for unreachable server: %v (%v)", code, err)
	}
}

func TestDialStdioInvalidBinary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, _, err := grpcclient.DialStdio(ctx, filepath.Join(t.TempDir(), "missing-binary")); err == nil {
		t.Fatal("expected DialStdio to fail for a missing binary")
	}
}

func TestDialStdioServerDidNotStart(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "exit-immediately.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write test script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, err := grpcclient.DialStdio(ctx, scriptPath)
	if err == nil {
		t.Fatal("expected DialStdio startup failure")
	}
	if !strings.Contains(err.Error(), "server did not start") &&
		!strings.Contains(err.Error(), "grpc handshake over stdio") &&
		!strings.Contains(err.Error(), "server startup timeout") {
		t.Fatalf("expected startup failure, got: %v", err)
	}
}

func TestDialStdioStartupTimeout(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "sleep.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatalf("write test script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, _, err := grpcclient.DialStdio(ctx, scriptPath)
	if err == nil {
		t.Fatal("expected DialStdio timeout")
	}
	if !strings.Contains(err.Error(), "server startup timeout") {
		t.Fatalf("expected startup timeout, got: %v", err)
	}
}

func startEchoGRPCServer(t *testing.T, lis net.Listener) {
	t.Helper()

	srv := grpc.NewServer()
	testgrpc.RegisterTestServiceServer(srv, &echoTestServer{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(lis)
	}()

	t.Cleanup(func() {
		srv.GracefulStop()
		_ = lis.Close()
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) &&
				!strings.Contains(err.Error(), "use of closed network connection") {
				t.Fatalf("server exit error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for gRPC server shutdown")
		}
	})
}

func requireUnaryEchoEventually(t *testing.T, conn *grpc.ClientConn, msg string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		out, err := unaryEcho(ctx, conn, msg)
		cancel()
		if err == nil {
			if out != msg {
				t.Fatalf("echo mismatch: got %q want %q", out, msg)
			}
			return
		}
		lastErr = err
		time.Sleep(30 * time.Millisecond)
	}

	t.Fatalf("echo RPC did not succeed before deadline: %v", lastErr)
}

func unaryEcho(ctx context.Context, conn *grpc.ClientConn, msg string) (string, error) {
	client := testgrpc.NewTestServiceClient(conn)
	resp, err := client.UnaryCall(ctx, &testgrpc.SimpleRequest{
		Payload: &testgrpc.Payload{
			Type: testgrpc.PayloadType_COMPRESSABLE,
			Body: []byte(msg),
		},
	})
	if err != nil {
		return "", err
	}
	return string(resp.GetPayload().GetBody()), nil
}

func requireStreamEcho(t *testing.T, conn *grpc.ClientConn, messages []string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := testgrpc.NewTestServiceClient(conn)
	stream, err := client.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("full duplex call: %v", err)
	}

	for _, msg := range messages {
		if err := stream.Send(&testgrpc.StreamingOutputCallRequest{
			Payload: &testgrpc.Payload{
				Type: testgrpc.PayloadType_COMPRESSABLE,
				Body: []byte(msg),
			},
		}); err != nil {
			t.Fatalf("stream send %q: %v", msg, err)
		}

		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("stream recv %q: %v", msg, err)
		}
		got := string(resp.GetPayload().GetBody())
		if got != msg {
			t.Fatalf("stream echo mismatch: got %q want %q", got, msg)
		}
	}

	if err := stream.CloseSend(); err != nil {
		t.Fatalf("stream close send: %v", err)
	}
}

func writeSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	return certFile, keyFile
}

func parseSleepMillis(payload []byte) time.Duration {
	const prefix = "sleep-ms:"
	if !bytes.HasPrefix(payload, []byte(prefix)) {
		return 0
	}
	millis, err := strconv.Atoi(strings.TrimPrefix(string(payload), prefix))
	if err != nil || millis <= 0 {
		return 0
	}
	return time.Duration(millis) * time.Millisecond
}
