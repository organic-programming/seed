package server_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"nhooyr.io/websocket"

	pb "github.com/organic-programming/examples/hello-world/gabriel-greet-go/gen/go/hello/v1"
	"github.com/organic-programming/examples/hello-world/gabriel-greet-go/pkg/server"
	"github.com/organic-programming/go-holons/pkg/transport"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// startBufconnServer launches a gRPC server over an in-memory bufconn.
func startBufconnServer(t *testing.T) (pb.HelloServiceClient, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	pb.RegisterHelloServiceServer(s, &server.Server{})
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		conn.Close()
		s.Stop()
	}

	return pb.NewHelloServiceClient(conn), cleanup
}

// --- Contract tests ---

func TestGreetWithName(t *testing.T) {
	client, cleanup := startBufconnServer(t)
	defer cleanup()

	resp, err := client.Greet(context.Background(), &pb.GreetRequest{Name: "Alice"})
	if err != nil {
		t.Fatalf("Greet failed: %v", err)
	}
	if resp.Message != "Hello Alice" {
		t.Errorf("Message = %q, want %q", resp.Message, "Hello Alice")
	}
}

func TestGreetEmpty(t *testing.T) {
	client, cleanup := startBufconnServer(t)
	defer cleanup()

	resp, err := client.Greet(context.Background(), &pb.GreetRequest{})
	if err != nil {
		t.Fatalf("Greet failed: %v", err)
	}
	if resp.Message != "Hello World" {
		t.Errorf("Message = %q, want %q", resp.Message, "Hello World")
	}
}

func TestGreetUnicode(t *testing.T) {
	client, cleanup := startBufconnServer(t)
	defer cleanup()

	resp, err := client.Greet(context.Background(), &pb.GreetRequest{Name: "世界"})
	if err != nil {
		t.Fatalf("Greet failed: %v", err)
	}
	if resp.Message != "Hello 世界" {
		t.Errorf("Message = %q, want %q", resp.Message, "Hello 世界")
	}
}

// --- mem:// transport test ---

func TestMemTransport(t *testing.T) {
	mem := transport.NewMemListener()
	s := grpc.NewServer()
	pb.RegisterHelloServiceServer(s, &server.Server{})
	go func() { _ = s.Serve(mem) }()
	defer s.Stop()

	conn, err := grpc.NewClient(
		"passthrough:///mem",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return mem.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewHelloServiceClient(conn)
	resp, err := client.Greet(context.Background(), &pb.GreetRequest{Name: "Mem"})
	if err != nil {
		t.Fatalf("Greet over mem://: %v", err)
	}
	if resp.Message != "Hello Mem" {
		t.Errorf("Message = %q, want %q", resp.Message, "Hello Mem")
	}
}

// --- ws:// transport test ---

func TestWSTransport(t *testing.T) {
	wsLis, err := transport.Listen("ws://127.0.0.1:0")
	if err != nil {
		t.Fatalf("ws listen: %v", err)
	}
	defer wsLis.Close()

	s := grpc.NewServer()
	pb.RegisterHelloServiceServer(s, &server.Server{})
	reflection.Register(s)
	go func() { _ = s.Serve(wsLis) }()
	defer s.Stop()

	wsAddr := wsLis.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsAddr, &websocket.DialOptions{
		Subprotocols: []string{"grpc"},
	})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	wsConn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	dialed := false
	//nolint:staticcheck
	conn, err := grpc.DialContext(ctx,
		"passthrough:///ws",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			if dialed {
				return nil, fmt.Errorf("already consumed")
			}
			dialed = true
			return wsConn, nil
		}),
		grpc.WithBlock(),
	)
	if err != nil {
		wsConn.Close()
		t.Fatalf("grpc dial over ws: %v", err)
	}
	defer conn.Close()

	client := pb.NewHelloServiceClient(conn)
	resp, err := client.Greet(context.Background(), &pb.GreetRequest{Name: "WebSocket"})
	if err != nil {
		t.Fatalf("Greet over ws://: %v", err)
	}
	if resp.Message != "Hello WebSocket" {
		t.Errorf("Message = %q, want %q", resp.Message, "Hello WebSocket")
	}
}

// --- ListenAndServe port conflict test ---

func TestListenAndServePortConflict(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	port := lis.Addr().(*net.TCPAddr).Port
	err = server.ListenAndServe(fmt.Sprintf("tcp://:%d", port), true)
	if err == nil {
		t.Fatal("expected error for port conflict")
	}
}
