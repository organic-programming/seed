package server_test

import (
	"context"
	"net"
	"testing"

	pb "github.com/organic-programming/seed/recipes/composition/workers/charon-worker-compute/gen/go/compute/v1"
	"github.com/organic-programming/seed/recipes/composition/workers/charon-worker-compute/internal/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func startServer(t *testing.T) pb.ComputeServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterComputeServiceServer(srv, &server.Server{})

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
	return pb.NewComputeServiceClient(conn)
}

func TestCompute(t *testing.T) {
	client := startServer(t)
	resp, err := client.Compute(context.Background(), &pb.ComputeRequest{Value: 7})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if got, want := resp.GetResult(), int64(49); got != want {
		t.Fatalf("result = %d, want %d", got, want)
	}
}
