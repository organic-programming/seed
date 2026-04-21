package internal_test

import (
	"context"
	"math"
	"net"
	"testing"

	pb "matt-calculator-go/gen/go/calculator/v1"
	"matt-calculator-go/internal"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// startServer launches a fresh CalculatorService on an in-memory connection.
// Each call creates a new independent accumulator at 0.0.
func startServer(t *testing.T) pb.CalculatorServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	acc := &internal.Accumulator{}
	pb.RegisterCalculatorServiceServer(s, &internal.Server{Acc: acc})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("server exited: %v", err)
		}
	}()
	t.Cleanup(s.GracefulStop)

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
	return pb.NewCalculatorServiceClient(conn)
}

func approxEqual(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

// TestDemoSequence runs the canonical five-operation demo and verifies each
// intermediate result and expression.
func TestDemoSequence(t *testing.T) {
	client := startServer(t)
	ctx := context.Background()

	steps := []struct {
		call   func() (*pb.CalcResponse, error)
		result float64
		expr   string
	}{
		{
			call:   func() (*pb.CalcResponse, error) { return client.Set(ctx, &pb.SetRequest{Value: 20}) },
			result: 20,
			expr:   "set → 20",
		},
		{
			call:   func() (*pb.CalcResponse, error) { return client.Add(ctx, &pb.AddSubRequest{Value: 1}) },
			result: 21,
			expr:   "20 + 1 = 21",
		},
		{
			call:   func() (*pb.CalcResponse, error) { return client.Subtract(ctx, &pb.AddSubRequest{Value: 4}) },
			result: 17,
			expr:   "21 - 4 = 17",
		},
		{
			call:   func() (*pb.CalcResponse, error) { return client.Divide(ctx, &pb.MulDivRequest{By: 5}) },
			result: 3.4,
			expr:   "17 / 5 = 3.4",
		},
		{
			call:   func() (*pb.CalcResponse, error) { return client.Multiply(ctx, &pb.MulDivRequest{By: 3}) },
			result: 10.2,
			expr:   "3.4 × 3 = 10.2",
		},
	}

	for i, step := range steps {
		resp, err := step.call()
		if err != nil {
			t.Fatalf("step %d: %v", i+1, err)
		}
		approxEqual(t, "step result", resp.Result, step.result)
		if resp.Expression != step.expr {
			t.Errorf("step %d: expression = %q, want %q", i+1, resp.Expression, step.expr)
		}
	}
}

// TestDivideByZero verifies that Divide(0) returns codes.InvalidArgument.
func TestDivideByZero(t *testing.T) {
	client := startServer(t)
	_, err := client.Divide(context.Background(), &pb.MulDivRequest{By: 0})
	if err == nil {
		t.Fatal("expected error for Divide(0), got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", st.Code())
	}
}

// TestAccumulatorIsPerServer verifies that two separately instantiated servers
// do not share state, mirroring what two separate op invoke calls would observe.
func TestAccumulatorIsPerServer(t *testing.T) {
	clientA := startServer(t)
	clientB := startServer(t)
	ctx := context.Background()

	respA, err := clientA.Add(ctx, &pb.AddSubRequest{Value: 10})
	if err != nil {
		t.Fatalf("clientA.Add: %v", err)
	}
	respB, err := clientB.Add(ctx, &pb.AddSubRequest{Value: 10})
	if err != nil {
		t.Fatalf("clientB.Add: %v", err)
	}
	approxEqual(t, "clientA result", respA.Result, 10)
	approxEqual(t, "clientB result", respB.Result, 10)
}
