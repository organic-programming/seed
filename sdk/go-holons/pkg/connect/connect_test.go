package connect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/discover"
	"github.com/organic-programming/go-holons/pkg/transport"
	"google.golang.org/grpc"
	testgrpc "google.golang.org/grpc/interop/grpc_testing"
)

type connectTestServer struct {
	testgrpc.UnimplementedTestServiceServer
	holonsv1.UnimplementedHolonMetaServer
}

func (s *connectTestServer) UnaryCall(_ context.Context, in *testgrpc.SimpleRequest) (*testgrpc.SimpleResponse, error) {
	payload := in.GetPayload()
	if payload == nil {
		payload = &testgrpc.Payload{
			Type: testgrpc.PayloadType_COMPRESSABLE,
			Body: []byte("connect-echo"),
		}
	}

	return &testgrpc.SimpleResponse{
		Payload: &testgrpc.Payload{
			Type: payload.GetType(),
			Body: append([]byte(nil), payload.GetBody()...),
		},
	}, nil
}

func (s *connectTestServer) Describe(context.Context, *holonsv1.DescribeRequest) (*holonsv1.DescribeResponse, error) {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Uuid:       "connect-test-uuid",
				GivenName:  "Connect",
				FamilyName: "Test",
				Status:     "draft",
			},
			Lang:      "go",
			Kind:      "native",
			Transport: "tcp",
			Build: &holonsv1.HolonManifest_Build{
				Runner: "go-module",
			},
			Artifacts: &holonsv1.HolonManifest_Artifacts{
				Binary: "connect-test",
			},
		},
	}, nil
}

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		os.Exit(runServeProcess(os.Args[2:]))
	}

	os.Exit(m.Run())
}

func TestConnectDirectTCP(t *testing.T) {
	lis, err := transport.Listen("tcp://127.0.0.1:0")
	if err != nil {
		skipIfLocalBindDenied(t, err)
		t.Fatalf("listen tcp: %v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })

	server := grpc.NewServer()
	testgrpc.RegisterTestServiceServer(server, &connectTestServer{})
	holonsv1.RegisterHolonMetaServer(server, &connectTestServer{})
	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- server.Serve(lis)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = <-serveErrCh
	})

	target := "tcp://" + lis.Addr().String()
	result := Connect(discover.LOCAL, target, nil, discover.ALL, 5000)
	if result.Error != "" {
		t.Fatalf("Connect error = %q", result.Error)
	}
	if result.Channel == nil {
		t.Fatal("expected channel")
	}
	if result.Origin == nil || result.Origin.URL != target {
		t.Fatalf("origin = %#v, want url %q", result.Origin, target)
	}
	defer func() {
		if err := Disconnect(result); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
	}()

	requireUnaryEcho(t, result.Channel, "direct-connect")
}

func TestConnectUnresolvable(t *testing.T) {
	root := t.TempDir()
	result := Connect(discover.LOCAL, "missing", &root, discover.ALL, 1000)
	if result.Error == "" {
		t.Fatal("expected connect error")
	}
}

func TestConnectResolvesAndDials(t *testing.T) {
	root, slug, _ := writeInstalledPackageFixture(t, "known-slug")

	result := Connect(discover.LOCAL, slug, &root, discover.INSTALLED, 5000)
	if result.Error != "" {
		t.Fatalf("Connect error = %q", result.Error)
	}
	if result.Channel == nil {
		t.Fatal("expected channel")
	}
	handle := lookupHandle(t, result.Channel)
	if got, want := handle.process.cmd.Args[1:], []string{"serve", "--listen", "stdio://"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("child args = %v, want %v", got, want)
	}
	defer func() {
		if err := Disconnect(result); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
	}()

	requireUnaryEcho(t, result.Channel, "installed-connect")
}

func TestConnectReturnsOrigin(t *testing.T) {
	root, slug, packageRoot := writeInstalledPackageFixture(t, "origin-slug")

	result := Connect(discover.LOCAL, slug, &root, discover.INSTALLED, 5000)
	if result.Error != "" {
		t.Fatalf("Connect error = %q", result.Error)
	}
	defer func() {
		if err := Disconnect(result); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
	}()

	if result.Origin == nil {
		t.Fatal("expected origin")
	}
	if result.Origin.Info == nil || result.Origin.Info.Slug != slug {
		t.Fatalf("origin info = %#v", result.Origin.Info)
	}
	if got, want := result.Origin.URL, "file://"+filepath.ToSlash(packageRoot); got != want {
		t.Fatalf("origin url = %q, want %q", got, want)
	}
}

func TestConnectUsesTransportHintWebSocket(t *testing.T) {
	root, slug, _ := writeInstalledPackageFixtureWithTransport(t, "ws-slug", "ws")

	result := Connect(discover.LOCAL, slug, &root, discover.INSTALLED, 5000)
	if result.Error != "" {
		t.Fatalf("Connect error = %q", result.Error)
	}
	if result.Channel == nil {
		t.Fatal("expected channel")
	}
	handle := lookupHandle(t, result.Channel)
	if got, want := handle.process.cmd.Args[1:], []string{"serve", "--listen", "ws://127.0.0.1:0/grpc"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("child args = %v, want %v", got, want)
	}
	defer func() {
		if err := Disconnect(result); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
	}()

	if result.Origin == nil || !strings.HasPrefix(result.Origin.URL, "ws://127.0.0.1:") {
		t.Fatalf("origin = %#v, want ws launch url", result.Origin)
	}
	requireUnaryEcho(t, result.Channel, "ws-connect")
}

func requireUnaryEcho(t *testing.T, conn *grpc.ClientConn, message string) {
	t.Helper()

	client := testgrpc.NewTestServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.UnaryCall(ctx, &testgrpc.SimpleRequest{
		Payload: &testgrpc.Payload{
			Type: testgrpc.PayloadType_COMPRESSABLE,
			Body: []byte(message),
		},
	})
	if err != nil {
		t.Fatalf("UnaryCall: %v", err)
	}

	if got := string(resp.GetPayload().GetBody()); got != message {
		t.Fatalf("UnaryCall body = %q, want %q", got, message)
	}
}

func writeInstalledPackageFixture(t *testing.T, slug string) (string, string, string) {
	return writeInstalledPackageFixtureWithTransport(t, slug, "")
}

func writeInstalledPackageFixtureWithTransport(t *testing.T, slug string, transport string) (string, string, string) {
	t.Helper()

	root := t.TempDir()
	opHome := filepath.Join(root, "runtime")
	opBin := filepath.Join(opHome, "bin")
	t.Setenv("OPPATH", opHome)
	t.Setenv("OPBIN", opBin)

	packageRoot := filepath.Join(opBin, slug+".holon")
	binaryPath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(packageRoot, "bin", packageArchDir()), 0o755); err != nil {
		t.Fatalf("mkdir package bin dir: %v", err)
	}
	if err := copyExecutable(binaryPath, filepath.Join(packageRoot, "bin", packageArchDir(), slug)); err != nil {
		t.Fatalf("copy executable: %v", err)
	}

	data := fmt.Sprintf(`{
  "schema": "holon-package/v1",
  "slug": %q,
  "uuid": %q,
  "identity": {
    "given_name": %q,
    "family_name": "Fixture"
  },
  "lang": "go",
  "runner": "go-module",
  "status": "draft",
  "kind": "native",
  "transport": %q,
  "entrypoint": %q,
  "architectures": [%q],
  "has_dist": false,
  "has_source": false
}
`, slug, slug+"-uuid", strings.Title(strings.ReplaceAll(slug, "-", " ")), transport, slug, packageArchDir())
	if err := os.WriteFile(filepath.Join(packageRoot, ".holon.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("write .holon.json: %v", err)
	}

	return root, slug, packageRoot
}

func runServeProcess(args []string) int {
	listenURI := "tcp://127.0.0.1:0"
	for i := 0; i < len(args); i++ {
		if args[i] == "--listen" && i+1 < len(args) {
			listenURI = args[i+1]
			break
		}
	}

	lis, err := transport.Listen(listenURI)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer lis.Close()

	server := grpc.NewServer()
	testgrpc.RegisterTestServiceServer(server, &connectTestServer{})

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- server.Serve(lis)
	}()

	if !strings.HasPrefix(listenURI, "stdio://") {
		fmt.Fprintln(os.Stderr, advertisedTestURI(listenURI, lis.Addr()))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	var serveErr error
	select {
	case <-sigCh:
		server.GracefulStop()
		serveErr = <-serveErrCh
	case serveErr = <-serveErrCh:
	}

	if isBenignServeError(serveErr) {
		return 0
	}

	fmt.Fprintln(os.Stderr, serveErr)
	return 1
}

func publicURI(listenURI string, addr net.Addr) string {
	host := "127.0.0.1"
	if strings.HasPrefix(listenURI, "tcp://") {
		rawHost := strings.TrimPrefix(listenURI, "tcp://")
		parsedHost, _, err := net.SplitHostPort(rawHost)
		if err == nil && parsedHost != "" && parsedHost != "0.0.0.0" && parsedHost != "::" && parsedHost != "[::]" {
			host = parsedHost
		}
	}

	_, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "tcp://" + addr.String()
	}
	return fmt.Sprintf("tcp://%s:%s", host, port)
}

func advertisedTestURI(listenURI string, addr net.Addr) string {
	if addr == nil {
		return listenURI
	}
	raw := strings.TrimSpace(addr.String())
	if raw == "" {
		return listenURI
	}
	if strings.Contains(raw, "://") {
		return raw
	}
	switch {
	case strings.HasPrefix(listenURI, "tcp://"):
		return publicURI(listenURI, addr)
	case strings.HasPrefix(listenURI, "unix://"):
		return "unix://" + raw
	case strings.HasPrefix(listenURI, "ws://"), strings.HasPrefix(listenURI, "wss://"):
		scheme := "ws://"
		if strings.HasPrefix(listenURI, "wss://") {
			scheme = "wss://"
		}
		path := "/grpc"
		if parsed, err := url.Parse(listenURI); err == nil && parsed.Path != "" {
			path = parsed.Path
		}
		return scheme + raw + path
	default:
		return listenURI
	}
}

func isBenignServeError(err error) bool {
	if err == nil ||
		errors.Is(err, grpc.ErrServerStopped) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

func skipIfLocalBindDenied(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}
	msg := strings.ToLower(err.Error())
	if errors.Is(err, syscall.EPERM) || strings.Contains(msg, "operation not permitted") {
		t.Skipf("local bind denied in this environment: %v", err)
	}
}

func copyExecutable(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()|fs.FileMode(0o111))
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func lookupHandle(t *testing.T, conn *grpc.ClientConn) connHandle {
	t.Helper()

	mu.Lock()
	handle, ok := started[conn]
	mu.Unlock()
	if !ok || handle.process == nil || handle.process.cmd == nil || handle.process.cmd.Process == nil {
		t.Fatal("expected started process handle")
	}
	return handle
}
