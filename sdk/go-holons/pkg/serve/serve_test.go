package serve_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/holonrpc"
	"github.com/organic-programming/go-holons/pkg/observability"
	"github.com/organic-programming/go-holons/pkg/serve"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	testgrpc "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
)

type serveEchoServer struct {
	testgrpc.UnimplementedTestServiceServer
}

func (s *serveEchoServer) EmptyCall(context.Context, *testgrpc.Empty) (*testgrpc.Empty, error) {
	return &testgrpc.Empty{}, nil
}

func (s *serveEchoServer) UnaryCall(ctx context.Context, in *testgrpc.SimpleRequest) (*testgrpc.SimpleResponse, error) {
	payload := in.GetPayload()
	if payload == nil {
		payload = &testgrpc.Payload{Type: testgrpc.PayloadType_COMPRESSABLE, Body: []byte("echo")}
	}

	_ = grpc.SetHeader(ctx, metadata.Pairs("x-holon", "go-holons"))
	_ = grpc.SetTrailer(ctx, metadata.Pairs("x-holon-trailer", "done"))

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

func (s *serveEchoServer) FullDuplexCall(stream testgrpc.TestService_FullDuplexCallServer) error {
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

func (s *serveEchoServer) StreamingOutputCall(
	in *testgrpc.StreamingOutputCallRequest,
	stream grpc.ServerStreamingServer[testgrpc.StreamingOutputCallResponse],
) error {
	payload := in.GetPayload()
	if payload == nil {
		payload = &testgrpc.Payload{Type: testgrpc.PayloadType_COMPRESSABLE, Body: []byte("echo")}
	}
	return stream.Send(&testgrpc.StreamingOutputCallResponse{
		Payload: &testgrpc.Payload{
			Type: payload.GetType(),
			Body: append([]byte(nil), payload.GetBody()...),
		},
	})
}

func TestServeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_SERVE_HELPER") != "1" {
		t.Skip("serve helper process")
	}

	helperArgs := helperProcessArgs(os.Args)
	if len(helperArgs) < 3 {
		fmt.Fprintf(os.Stderr, "serve helper expects: <mode> <listen-uri> <reflect> [<listen-uri>...]\n")
		os.Exit(2)
	}

	mode := helperArgs[0]
	reflectEnabled, err := strconv.ParseBool(helperArgs[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid reflect value %q: %v\n", helperArgs[2], err)
		os.Exit(2)
	}
	listenURIs := append([]string{helperArgs[1]}, helperArgs[3:]...)

	register := func(s *grpc.Server) {
		testgrpc.RegisterTestServiceServer(s, &serveEchoServer{})
	}

	switch mode {
	case "run":
		describe.UseStaticResponse(serveStaticDescribeResponse())
		err = serve.Run(listenURIs[0], register, listenURIs[1:]...)
	case "run-with-options":
		describe.UseStaticResponse(serveStaticDescribeResponse())
		err = serve.RunWithOptions(listenURIs[0], register, reflectEnabled, listenURIs[1:]...)
	case "run-with-member":
		describe.UseStaticResponse(serveStaticDescribeResponse())
		options := serve.ServeOptions{Reflect: reflectEnabled}
		if address := os.Getenv("GO_SERVE_MEMBER_ADDRESS"); address != "" {
			options.MemberEndpoints = append(options.MemberEndpoints, serve.MemberRef{
				Slug:    os.Getenv("GO_SERVE_MEMBER_SLUG"),
				UID:     os.Getenv("GO_SERVE_MEMBER_UID"),
				Address: address,
			})
		}
		err = serve.RunWithServeOptions(listenURIs[0], register, options, listenURIs[1:]...)
	case "run-empty":
		err = serve.RunWithOptions(listenURIs[0], func(*grpc.Server) {}, reflectEnabled, listenURIs[1:]...)
	case "run-empty-static":
		describe.UseStaticResponse(serveStaticDescribeResponse())
		err = serve.RunWithOptions(listenURIs[0], func(*grpc.Server) {}, reflectEnabled, listenURIs[1:]...)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper mode %q\n", mode)
		os.Exit(2)
	}

	if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	os.Exit(0)
}

func TestRunServesGRPCOnRandomPort(t *testing.T) {
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cmd, logs := startServeProcess(t, "run", listenURI, true)
	defer stopServeProcess(t, cmd, logs)

	conn := dialServeAndWait(t, address)
	defer conn.Close()

	requireUnaryEchoEventually(t, conn, "serve-run")
	requireStreamEchoEventually(t, conn, []string{"serve-stream-1", "serve-stream-2"})
}

func TestRunAdvertisesResolvedRandomPort(t *testing.T) {
	cmd, logs := startServeProcess(t, "run", "tcp://127.0.0.1:0", true)
	defer stopServeProcess(t, cmd, logs)

	address := waitForAdvertisedAddress(t, logs, "tcp://127.0.0.1:")
	if strings.HasSuffix(address, ":0") {
		t.Fatalf("advertised unresolved address: %q\nlogs:\n%s", address, logs.String())
	}

	conn := dialServeAndWait(t, strings.TrimPrefix(address, "tcp://"))
	defer conn.Close()

	requireUnaryEchoEventually(t, conn, "serve-run-random-port")
}

func TestRunServesGRPCOnUnixSocket(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("serve-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(socketPath) })

	listenURI := "unix://" + socketPath
	cmd, logs := startServeProcess(t, "run", listenURI, false)
	defer stopServeProcess(t, cmd, logs)

	conn := dialServeAndWait(t, listenURI)
	defer conn.Close()

	requireUnaryEchoEventually(t, conn, "serve-unix")
	requireStreamEchoEventually(t, conn, []string{"serve-unix-stream-1", "serve-unix-stream-2"})
}

func TestRunServesGRPCOnWebSocket(t *testing.T) {
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("ws://127.0.0.1:%d/grpc", port)

	cmd, logs := startServeProcess(t, "run", listenURI, false)
	defer stopServeProcess(t, cmd, logs)

	conn := dialServeWebSocketAndWait(t, listenURI)
	defer conn.Close()

	requireUnaryEchoEventually(t, conn, "serve-ws")
	requireStreamEchoEventually(t, conn, []string{"serve-ws-stream-1", "serve-ws-stream-2"})
}

func TestRunServesHTTPRPCAndSSE(t *testing.T) {
	cmd, logs := startServeProcess(t, "run", "http://127.0.0.1:0/api/v1/rpc", false)
	defer stopServeProcess(t, cmd, logs)

	address := waitForAdvertisedAddress(t, logs, "http://127.0.0.1:")
	client := holonrpc.NewHTTPClient(address)

	requireHTTPDescribeEventually(t, client, "echo-server")
	requireHTTPUnaryEchoEventually(t, client, "serve-http")
	requireHTTPStreamEchoEventually(t, client, "serve-http-stream")
}

func TestRunServesGRPCAndHTTPConcurrently(t *testing.T) {
	port := freeTCPPort(t)
	grpcListenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	httpListenURI := "http://127.0.0.1:0/api/v1/rpc"

	cmd, logs := startServeProcess(t, "run", grpcListenURI, false, httpListenURI)
	defer stopServeProcess(t, cmd, logs)

	conn := dialServeAndWait(t, fmt.Sprintf("127.0.0.1:%d", port))
	defer conn.Close()
	requireUnaryEchoEventually(t, conn, "serve-multi-grpc")

	address := waitForAdvertisedAddress(t, logs, "http://127.0.0.1:")
	client := holonrpc.NewHTTPClient(address)
	requireHTTPDescribeEventually(t, client, "echo-server")
	requireHTTPUnaryEchoEventually(t, client, "serve-multi-http")
	requireHTTPStreamEchoEventually(t, client, "serve-multi-stream")
}

func TestRunWithServeOptionsStartsRelayAndMultilog(t *testing.T) {
	observability.Reset()
	defer observability.Reset()
	t.Setenv("OP_OBS", "logs,events")
	childObs := observability.Configure(observability.Config{
		Slug:        "child-holon",
		InstanceUID: "child-uid",
	})
	defer childObs.Close()
	childObs.Logger("child").Info("child ready")

	childServer := grpc.NewServer()
	holonsv1.RegisterHolonObservabilityServer(childServer, observability.NewService(childObs, observability.VisibilityFull))
	childLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("child listen: %v", err)
	}
	go childServer.Serve(childLis)
	defer childServer.Stop()

	runRoot := t.TempDir()
	rootUID := "root-uid"
	rootSlug := filepath.Base(os.Args[0])
	t.Setenv("OP_OBS", "logs,events")
	t.Setenv("OP_RUN_DIR", runRoot)
	t.Setenv("OP_INSTANCE_UID", rootUID)
	t.Setenv("OP_ORGANISM_UID", rootUID)
	t.Setenv("OP_ORGANISM_SLUG", rootSlug)
	t.Setenv("GO_SERVE_MEMBER_SLUG", "child-holon")
	t.Setenv("GO_SERVE_MEMBER_UID", "child-uid")
	t.Setenv("GO_SERVE_MEMBER_ADDRESS", "tcp://"+childLis.Addr().String())

	cmd, logs := startServeProcess(t, "run-with-member", "tcp://127.0.0.1:0", false)
	defer stopServeProcess(t, cmd, logs)
	_ = waitForAdvertisedAddress(t, logs, "tcp://127.0.0.1:")

	multilogPath := filepath.Join(runRoot, rootSlug, rootUID, "multilog.jsonl")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		records, err := observability.ReadMultilog(multilogPath)
		if err == nil && multilogHasChainDepth(records, "child-holon", 2) && multilogHasChainDepth(records, rootSlug, 1) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	records, _ := observability.ReadMultilog(multilogPath)
	t.Fatalf("multilog missing relayed/root enriched chains at %s; logs:\n%s\nrecords:%+v", multilogPath, logs.String(), records)
}

func multilogHasChainDepth(records []map[string]any, slug string, depth int) bool {
	for _, rec := range records {
		if rec["slug"] != slug {
			continue
		}
		chain, ok := rec["chain"].([]any)
		if ok && len(chain) == depth {
			return true
		}
	}
	return false
}

func TestParseFlags(t *testing.T) {
	testCases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "listen-flag",
			args: []string{"--listen", "unix:///tmp/holon.sock"},
			want: "unix:///tmp/holon.sock",
		},
		{
			name: "legacy-port-flag",
			args: []string{"--port", "7070"},
			want: "tcp://:7070",
		},
		{
			name: "default",
			args: []string{"--unknown", "value"},
			want: "tcp://:9090",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := serve.ParseFlags(tc.args); got != tc.want {
				t.Fatalf("ParseFlags(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestParseOptions(t *testing.T) {
	got := serve.ParseOptions([]string{"--listen", "tcp://:8080", "--listen", "http://127.0.0.1:8080/api/v1/rpc", "--reflect"})
	if got.ListenURI != "tcp://:8080" {
		t.Fatalf("ListenURI = %q, want %q", got.ListenURI, "tcp://:8080")
	}
	if want := []string{"tcp://:8080", "http://127.0.0.1:8080/api/v1/rpc"}; !equalStrings(got.ListenURIs, want) {
		t.Fatalf("ListenURIs = %v, want %v", got.ListenURIs, want)
	}
	if !got.Reflect {
		t.Fatal("Reflect = false, want true")
	}
}

func TestRunDefaultsReflectionOff(t *testing.T) {
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cmd, logs := startServeProcess(t, "run", listenURI, false)
	defer stopServeProcess(t, cmd, logs)

	conn := dialServeAndWait(t, address)
	defer conn.Close()

	requireUnaryEchoEventually(t, conn, "serve-run-default-reflection-off")
	requireReflectionState(t, conn, false)
}

func TestRunWithOptionsReflectionToggle(t *testing.T) {
	testCases := []struct {
		name            string
		reflectEnabled  bool
		expectSupported bool
	}{
		{name: "reflection-enabled", reflectEnabled: true, expectSupported: true},
		{name: "reflection-disabled", reflectEnabled: false, expectSupported: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			port := freeTCPPort(t)
			listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
			address := fmt.Sprintf("127.0.0.1:%d", port)

			cmd, logs := startServeProcess(t, "run-with-options", listenURI, tc.reflectEnabled)
			defer stopServeProcess(t, cmd, logs)

			conn := dialServeAndWait(t, address)
			defer conn.Close()

			requireUnaryEchoEventually(t, conn, tc.name)
			requireReflectionState(t, conn, tc.expectSupported)
		})
	}
}

func TestRunGracefulShutdownOnContextCancellation(t *testing.T) {
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cmd, logs := startServeProcess(t, "run-with-options", listenURI, false)
	conn := dialServeAndWait(t, address)
	_ = conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	signalErr := make(chan error, 1)
	go func() {
		<-ctx.Done()
		signalErr <- cmd.Process.Signal(syscall.SIGTERM)
	}()

	cancel()
	if err := <-signalErr; err != nil {
		stopServeProcess(t, cmd, logs)
		t.Fatalf("signal serve helper: %v", err)
	}

	if err := waitProcessExit(cmd, 5*time.Second); err != nil {
		stopServeProcess(t, cmd, logs)
		t.Fatalf("serve helper did not exit gracefully: %v\nlogs:\n%s", err, logs.String())
	}

	ctxDial, cancelDial := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancelDial()
	postConn, err := grpcclient.Dial(ctxDial, address)
	if err == nil {
		defer postConn.Close()
		rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer rpcCancel()
		if _, rpcErr := unaryEcho(rpcCtx, postConn, "after-stop"); rpcErr == nil {
			t.Fatalf("expected RPC to fail after graceful shutdown")
		}
	}
}

func TestRunGracefulShutdownWithInFlightRPC(t *testing.T) {
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cmd, logs := startServeProcess(t, "run", listenURI, false)
	conn := dialServeAndWait(t, address)
	defer conn.Close()

	inFlightErrCh := make(chan error, 1)
	go func() {
		callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer callCancel()
		_, invokeErr := unaryEcho(callCtx, conn, "sleep-ms:1200")
		inFlightErrCh <- invokeErr
	}()

	time.Sleep(150 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		stopServeProcess(t, cmd, logs)
		t.Fatalf("signal serve helper: %v", err)
	}
	if err := waitProcessExit(cmd, 5*time.Second); err != nil {
		stopServeProcess(t, cmd, logs)
		t.Fatalf("serve helper did not exit gracefully: %v\nlogs:\n%s", err, logs.String())
	}

	select {
	case invokeErr := <-inFlightErrCh:
		if invokeErr == nil {
			return
		}
		code := status.Code(invokeErr)
		if code == codes.Unavailable || code == codes.Canceled || code == codes.DeadlineExceeded {
			return
		}
		t.Fatalf("unexpected in-flight RPC error: %v", invokeErr)
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight RPC did not finish during shutdown")
	}
}

func TestRunConcurrentClients(t *testing.T) {
	const clients = 10

	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cmd, logs := startServeProcess(t, "run", listenURI, false)
	defer stopServeProcess(t, cmd, logs)

	conns := make([]*grpc.ClientConn, 0, clients)
	for i := 0; i < clients; i++ {
		conn := dialServeAndWait(t, address)
		conns = append(conns, conn)
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	errCh := make(chan error, clients)
	var wg sync.WaitGroup
	wg.Add(clients)
	for i := 0; i < clients; i++ {
		i := i
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			want := fmt.Sprintf("c-%d", i)
			got, err := unaryEcho(ctx, conns[i], want)
			if err != nil {
				errCh <- err
				return
			}
			if got != want {
				errCh <- fmt.Errorf("echo mismatch: got %q want %q", got, want)
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunMetadataHeadersAndTrailers(t *testing.T) {
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cmd, logs := startServeProcess(t, "run", listenURI, false)
	defer stopServeProcess(t, cmd, logs)

	conn := dialServeAndWait(t, address)
	defer conn.Close()

	client := testgrpc.NewTestServiceClient(conn)
	var header metadata.MD
	var trailer metadata.MD

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.UnaryCall(
		ctx,
		&testgrpc.SimpleRequest{
			Payload: &testgrpc.Payload{
				Type: testgrpc.PayloadType_COMPRESSABLE,
				Body: []byte("metadata-probe"),
			},
		},
		grpc.Header(&header),
		grpc.Trailer(&trailer),
	)
	if err != nil {
		t.Fatalf("unary metadata call: %v", err)
	}

	if got := header.Get("x-holon"); len(got) == 0 || got[0] != "go-holons" {
		t.Fatalf("header x-holon = %v, want [go-holons]", got)
	}
	if got := trailer.Get("x-holon-trailer"); len(got) == 0 || got[0] != "done" {
		t.Fatalf("trailer x-holon-trailer = %v, want [done]", got)
	}
}

func TestRunRejectsOversizedMessage(t *testing.T) {
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cmd, logs := startServeProcess(t, "run", listenURI, false)
	defer stopServeProcess(t, cmd, logs)

	conn := dialServeAndWait(t, address)
	defer conn.Close()

	client := testgrpc.NewTestServiceClient(conn)
	oversized := bytes.Repeat([]byte("a"), 5*1024*1024)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := client.UnaryCall(ctx, &testgrpc.SimpleRequest{
		Payload: &testgrpc.Payload{
			Type: testgrpc.PayloadType_COMPRESSABLE,
			Body: oversized,
		},
	})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected RESOURCE_EXHAUSTED, got %v (%v)", status.Code(err), err)
	}

	requireUnaryEchoEventually(t, conn, "post-oversize")
}

func TestRunRegistersStaticHolonMeta(t *testing.T) {
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cmd, logs := startServeProcess(t, "run-empty-static", listenURI, false)
	defer stopServeProcess(t, cmd, logs)

	conn := dialDescribeAndWait(t, address)
	defer conn.Close()

	client := holonsv1.NewHolonMetaClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	response, err := client.Describe(ctx, &holonsv1.DescribeRequest{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}

	if got := responseManifestSlug(response); got != "echo-server" {
		t.Fatalf("slug = %q, want %q", got, "echo-server")
	}
	if got := response.GetManifest().GetIdentity().GetMotto(); got != "Reply precisely." {
		t.Fatalf("motto = %q, want %q", got, "Reply precisely.")
	}
	if len(response.GetServices()) != 1 {
		t.Fatalf("services len = %d, want 1", len(response.GetServices()))
	}

	service := response.GetServices()[0]
	if service.GetName() != "grpc.testing.TestService" {
		t.Fatalf("service name = %q, want %q", service.GetName(), "grpc.testing.TestService")
	}
	if want := []string{"EmptyCall", "UnaryCall", "CacheableUnaryCall", "StreamingOutputCall", "UnimplementedCall"}; !equalStrings(methodNames(service.GetMethods()), want) {
		t.Fatalf("methods = %v, want %v", methodNames(service.GetMethods()), want)
	}
}

func TestRunFailsWithoutIncodeDescriptionEvenWhenProtoExists(t *testing.T) {
	workdir := describeEchoHolonDir(t)
	port := freeTCPPort(t)
	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", port)

	output, err := runServeProcessOnceInDir(t, workdir, "run-empty", listenURI, false)
	if err == nil {
		t.Fatalf("expected serve startup to fail, output:\n%s", output)
	}
	if !strings.Contains(output, describe.ErrNoIncodeDescription.Error()) {
		t.Fatalf("expected missing Incode Description error, got:\n%s", output)
	}
	if !strings.Contains(output, "HolonMeta registration failed") {
		t.Fatalf("expected explicit HolonMeta registration log, got:\n%s", output)
	}
}

func responseManifestSlug(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return ""
	}
	ident := response.GetManifest().GetIdentity()
	return strings.ToLower(strings.Trim(strings.ReplaceAll(ident.GetGivenName()+"-"+strings.TrimSuffix(ident.GetFamilyName(), "?"), " ", "-"), "-"))
}

func startServeProcess(t *testing.T, mode, listenURI string, reflectEnabled bool, moreListenURIs ...string) (*exec.Cmd, *lockedBuffer) {
	t.Helper()
	return startServeProcessInDir(t, "", mode, listenURI, reflectEnabled, moreListenURIs...)
}

func startServeProcessInDir(t *testing.T, dir, mode, listenURI string, reflectEnabled bool, moreListenURIs ...string) (*exec.Cmd, *lockedBuffer) {
	t.Helper()

	args := []string{"-test.run=TestServeHelperProcess", "--", mode, listenURI, strconv.FormatBool(reflectEnabled)}
	args = append(args, moreListenURIs...)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "GO_WANT_SERVE_HELPER=1")
	if dir != "" {
		cmd.Dir = dir
	}

	logs := &lockedBuffer{}
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		t.Fatalf("start serve helper: %v", err)
	}

	return cmd, logs
}

func runServeProcessOnceInDir(t *testing.T, dir, mode, listenURI string, reflectEnabled bool, moreListenURIs ...string) (string, error) {
	t.Helper()

	args := []string{"-test.run=TestServeHelperProcess", "--", mode, listenURI, strconv.FormatBool(reflectEnabled)}
	args = append(args, moreListenURIs...)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "GO_WANT_SERVE_HELPER=1")
	if dir != "" {
		cmd.Dir = dir
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.String(), err
}

func waitForAdvertisedAddress(t *testing.T, logs *lockedBuffer, prefix string) string {
	t.Helper()

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		for _, field := range strings.Fields(logs.String()) {
			if strings.HasPrefix(field, prefix) {
				return strings.TrimRight(field, ".,)")
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for advertised address with prefix %q\nlogs:\n%s", prefix, logs.String())
	return ""
}

func stopServeProcess(t *testing.T, cmd *exec.Cmd, logs *lockedBuffer) {
	t.Helper()

	if cmd == nil || cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	if err := waitProcessExit(cmd, 4*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("stop serve helper: %v\nlogs:\n%s", err, logs.String())
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func waitProcessExit(cmd *exec.Cmd, timeout time.Duration) error {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return fmt.Errorf("process exited with status %d", exitErr.ExitCode())
			}
			return err
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for process exit")
	}
}

func helperProcessArgs(args []string) []string {
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

func freeTCPPort(t *testing.T) int {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free port: %v", err)
	}
	defer lis.Close()

	addr, ok := lis.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener addr type: %T", lis.Addr())
	}
	return addr.Port
}

func dialServeAndWait(t *testing.T, address string) *grpc.ClientConn {
	t.Helper()

	deadline := time.Now().Add(6 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		conn, err := grpcclient.Dial(ctx, address)
		cancel()
		if err == nil {
			rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			_, rpcErr := unaryEcho(rpcCtx, conn, "probe")
			rpcCancel()
			if rpcErr == nil {
				return conn
			}
			lastErr = rpcErr
			_ = conn.Close()
		} else {
			lastErr = err
		}
		time.Sleep(40 * time.Millisecond)
	}

	t.Fatalf("server %s not ready: %v", address, lastErr)
	return nil
}

func requireReflectionState(t *testing.T, conn *grpc.ClientConn, enabled bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := reflectionv1alpha.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		if !enabled && status.Code(err) == codes.Unimplemented {
			return
		}
		t.Fatalf("reflection stream: %v", err)
	}

	req := &reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_ListServices{ListServices: "*"},
	}
	if err := stream.Send(req); err != nil {
		if !enabled && (status.Code(err) == codes.Unimplemented || errors.Is(err, io.EOF)) {
			return
		}
		t.Fatalf("reflection send: %v", err)
	}

	resp, err := stream.Recv()
	if !enabled {
		if status.Code(err) == codes.Unimplemented {
			return
		}
		t.Fatalf("expected reflection to be disabled, got response=%v err=%v", resp, err)
	}
	if err != nil {
		t.Fatalf("reflection recv: %v", err)
	}

	services := resp.GetListServicesResponse().GetService()
	for _, svc := range services {
		if svc.GetName() == "grpc.testing.TestService" {
			return
		}
	}
	t.Fatalf("reflection response missing grpc.testing.TestService")
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

func requireStreamEchoEventually(t *testing.T, conn *grpc.ClientConn, messages []string) {
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
		if got := string(resp.GetPayload().GetBody()); got != msg {
			t.Fatalf("stream echo mismatch: got %q want %q", got, msg)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("stream close send: %v", err)
	}
}

func requireHTTPDescribeEventually(t *testing.T, client *holonrpc.HTTPClient, wantSlug string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		response, err := client.Invoke(ctx, "holons.v1.HolonMeta/Describe", nil)
		cancel()
		if err == nil {
			if manifest, ok := response["manifest"].(map[string]any); ok {
				if identity, ok := manifest["identity"].(map[string]any); ok {
					given, _ := identity["givenName"].(string)
					family, _ := identity["familyName"].(string)
					got := strings.ToLower(strings.Trim(strings.ReplaceAll(given+"-"+strings.TrimSuffix(family, "?"), " ", "-"), "-"))
					if got == wantSlug {
						return
					}
					lastErr = fmt.Errorf("describe slug = %q, want %q", got, wantSlug)
				}
			}
		} else {
			lastErr = err
		}
		time.Sleep(30 * time.Millisecond)
	}

	t.Fatalf("HTTP Describe did not succeed before deadline: %v", lastErr)
}

func requireHTTPUnaryEchoEventually(t *testing.T, client *holonrpc.HTTPClient, message string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		result, err := client.Invoke(ctx, "grpc.testing.TestService/UnaryCall", map[string]any{
			"payload": map[string]any{
				"type": "COMPRESSABLE",
				"body": message,
			},
		})
		cancel()
		if err == nil {
			body, decodeErr := httpPayloadBody(result)
			if decodeErr == nil && body == message {
				return
			}
			if decodeErr != nil {
				lastErr = decodeErr
			} else {
				lastErr = fmt.Errorf("HTTP unary echo mismatch: got %q want %q", body, message)
			}
		} else {
			lastErr = err
		}
		time.Sleep(30 * time.Millisecond)
	}

	t.Fatalf("HTTP unary echo did not succeed before deadline: %v", lastErr)
}

func requireHTTPStreamEchoEventually(t *testing.T, client *holonrpc.HTTPClient, message string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		events, err := client.Stream(ctx, "grpc.testing.TestService/StreamingOutputCall", map[string]any{
			"payload": map[string]any{
				"type": "COMPRESSABLE",
				"body": message,
			},
		})
		cancel()
		if err == nil {
			if len(events) < 2 {
				lastErr = fmt.Errorf("HTTP stream events = %d, want at least 2", len(events))
				time.Sleep(30 * time.Millisecond)
				continue
			}
			if events[0].Event != "message" {
				lastErr = fmt.Errorf("first HTTP stream event = %q, want message", events[0].Event)
				time.Sleep(30 * time.Millisecond)
				continue
			}
			body, decodeErr := httpPayloadBody(events[0].Result)
			if decodeErr != nil {
				lastErr = decodeErr
				time.Sleep(30 * time.Millisecond)
				continue
			}
			if body != message {
				lastErr = fmt.Errorf("HTTP stream echo mismatch: got %q want %q", body, message)
				time.Sleep(30 * time.Millisecond)
				continue
			}
			if events[len(events)-1].Event != "done" {
				lastErr = fmt.Errorf("last HTTP stream event = %q, want done", events[len(events)-1].Event)
				time.Sleep(30 * time.Millisecond)
				continue
			}
			return
		}
		lastErr = err
		time.Sleep(30 * time.Millisecond)
	}

	t.Fatalf("HTTP stream echo did not succeed before deadline: %v", lastErr)
}

func httpPayloadBody(result map[string]any) (string, error) {
	payload, ok := result["payload"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("payload missing from HTTP result: %#v", result)
	}
	rawBody, ok := payload["body"].(string)
	if !ok {
		return "", fmt.Errorf("payload.body missing from HTTP result: %#v", payload["body"])
	}
	decoded, err := base64.StdEncoding.DecodeString(rawBody)
	if err == nil && utf8.Valid(decoded) {
		return string(decoded), nil
	}
	return rawBody, nil
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func methodNames(methods []*holonsv1.MethodDoc) []string {
	names := make([]string, 0, len(methods))
	for _, method := range methods {
		names = append(names, method.GetName())
	}
	return names
}

func dialServeWebSocketAndWait(t *testing.T, wsURI string) *grpc.ClientConn {
	t.Helper()

	deadline := time.Now().Add(6 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
		conn, err := grpcclient.DialWebSocket(ctx, wsURI)
		cancel()
		if err == nil {
			return conn
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("websocket server %s not ready: %v", wsURI, lastErr)
	return nil
}

func dialDescribeAndWait(t *testing.T, address string) *grpc.ClientConn {
	t.Helper()

	deadline := time.Now().Add(6 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		conn, err := grpcclient.Dial(ctx, address)
		cancel()
		if err != nil {
			lastErr = err
			time.Sleep(40 * time.Millisecond)
			continue
		}

		describeCtx, describeCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		_, describeErr := holonsv1.NewHolonMetaClient(conn).Describe(describeCtx, &holonsv1.DescribeRequest{})
		describeCancel()
		if describeErr == nil {
			return conn
		}

		lastErr = describeErr
		_ = conn.Close()
		time.Sleep(40 * time.Millisecond)
	}

	t.Fatalf("HolonMeta server %s not ready: %v", address, lastErr)
	return nil
}

func describeEchoHolonDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	copyServeTree(
		t,
		filepath.Join(serveDescribeTestdataRoot(t), "echoholon", "protos"),
		filepath.Join(root, "protos"),
	)
	writeServeSharedManifestProto(t, root)
	writeServeTestFile(t, filepath.Join(root, "protos", "echo", "v1", "holon.proto"), `syntax = "proto3";

package echo.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: "echo-server-0000"
    given_name: "Echo"
    family_name: "Server"
    motto: "Reply precisely."
    composer: "serve-test"
    status: "draft"
    born: "2026-03-17"
  }
  lang: "go"
};
`)
	return root
}

func parseSleepMillis(payload []byte) time.Duration {
	const prefix = "sleep-ms:"
	if !strings.HasPrefix(string(payload), prefix) {
		return 0
	}
	millis, err := strconv.Atoi(strings.TrimPrefix(string(payload), prefix))
	if err != nil || millis <= 0 {
		return 0
	}
	return time.Duration(millis) * time.Millisecond
}

func serveDescribeTestdataRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "describe", "testdata")
}

func serveRepoManifestProtoPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "identity", "testdata", "_protos", "holons", "v1", "manifest.proto")
}

func writeServeSharedManifestProto(t *testing.T, root string) {
	t.Helper()

	data, err := os.ReadFile(serveRepoManifestProtoPath(t))
	if err != nil {
		t.Fatalf("read manifest proto: %v", err)
	}
	writeServeTestFile(t, filepath.Join(root, "_protos", "holons", "v1", "manifest.proto"), string(data))
}

func copyServeTree(t *testing.T, src, dst string) {
	t.Helper()

	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}

func serveStaticDescribeResponse() *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Schema:     "holon/v1",
				Uuid:       "echo-server-0000",
				GivenName:  "Echo",
				FamilyName: "Server",
				Motto:      "Reply precisely.",
				Composer:   "serve-test",
				Status:     "draft",
				Born:       "2026-03-17",
			},
			Lang: "go",
		},
		Services: []*holonsv1.ServiceDoc{{
			Name:        "grpc.testing.TestService",
			Description: "Interop test service bridged into HTTP+SSE during serve tests.",
			Methods: []*holonsv1.MethodDoc{
				{
					Name:        "EmptyCall",
					Description: "Empty unary endpoint used by serve tests.",
					InputType:   "grpc.testing.Empty",
					OutputType:  "grpc.testing.Empty",
				},
				{
					Name:        "UnaryCall",
					Description: "Unary echo endpoint used by serve tests.",
					InputType:   "grpc.testing.SimpleRequest",
					OutputType:  "grpc.testing.SimpleResponse",
				},
				{
					Name:        "CacheableUnaryCall",
					Description: "Cacheable unary endpoint bridged for completeness in serve tests.",
					InputType:   "grpc.testing.SimpleRequest",
					OutputType:  "grpc.testing.SimpleResponse",
				},
				{
					Name:            "StreamingOutputCall",
					Description:     "Server-streaming echo endpoint used by serve tests.",
					InputType:       "grpc.testing.StreamingOutputCallRequest",
					OutputType:      "grpc.testing.StreamingOutputCallResponse",
					ServerStreaming: true,
				},
				{
					Name:        "UnimplementedCall",
					Description: "Explicitly unimplemented unary endpoint bridged for completeness in serve tests.",
					InputType:   "grpc.testing.Empty",
					OutputType:  "grpc.testing.Empty",
				},
			},
		}},
	}
}

func writeServeTestFile(t *testing.T, path string, data string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
