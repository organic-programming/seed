package server_test

import (
	"context"
	"net"
	"testing"

	pb "github.com/organic-programming/seed/recipes/composition/workers/charon-worker-transform/gen/go/transform/v1"
	"github.com/organic-programming/seed/recipes/composition/workers/charon-worker-transform/internal/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func startServer(t *testing.T) pb.TransformServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterTransformServiceServer(srv, &server.Server{})

	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("server exited: %v", err)
		}
	}()
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewTransformServiceClient(conn)
}

func TestTransform(t *testing.T) {
	client := startServer(t)
	resp, err := client.Transform(context.Background(), &pb.TransformRequest{Text: "stressed"})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if got, want := resp.GetResult(), "desserts"; got != want {
		t.Fatalf("result = %q, want %q", got, want)
	}
}
