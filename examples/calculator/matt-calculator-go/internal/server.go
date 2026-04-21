// Package internal contains the calculator accumulator and gRPC server.
package internal

import (
	"context"
	"fmt"

	"github.com/organic-programming/go-holons/pkg/serve"

	pb "matt-calculator-go/gen/go/calculator/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements CalculatorService with a process-scoped accumulator.
// Acc is exported so unit tests can inject a specific Accumulator instance.
type Server struct {
	pb.UnimplementedCalculatorServiceServer
	Acc *Accumulator
}

func (s *Server) Set(_ context.Context, req *pb.SetRequest) (*pb.CalcResponse, error) {
	result := s.Acc.Set(req.GetValue())
	return &pb.CalcResponse{
		Result:     result,
		Expression: fmt.Sprintf("set → %s", FormatFloat(result)),
	}, nil
}

func (s *Server) Add(_ context.Context, req *pb.AddSubRequest) (*pb.CalcResponse, error) {
	prev, next := s.Acc.Add(req.GetValue())
	return &pb.CalcResponse{
		Result:     next,
		Expression: fmt.Sprintf("%s + %s = %s", FormatFloat(prev), FormatFloat(req.GetValue()), FormatFloat(next)),
	}, nil
}

func (s *Server) Subtract(_ context.Context, req *pb.AddSubRequest) (*pb.CalcResponse, error) {
	prev, next := s.Acc.Subtract(req.GetValue())
	return &pb.CalcResponse{
		Result:     next,
		Expression: fmt.Sprintf("%s - %s = %s", FormatFloat(prev), FormatFloat(req.GetValue()), FormatFloat(next)),
	}, nil
}

func (s *Server) Multiply(_ context.Context, req *pb.MulDivRequest) (*pb.CalcResponse, error) {
	prev, next := s.Acc.Multiply(req.GetBy())
	return &pb.CalcResponse{
		Result:     next,
		Expression: fmt.Sprintf("%s × %s = %s", FormatFloat(prev), FormatFloat(req.GetBy()), FormatFloat(next)),
	}, nil
}

func (s *Server) Divide(_ context.Context, req *pb.MulDivRequest) (*pb.CalcResponse, error) {
	prev, next, err := s.Acc.Divide(req.GetBy())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return &pb.CalcResponse{
		Result:     next,
		Expression: fmt.Sprintf("%s / %s = %s", FormatFloat(prev), FormatFloat(req.GetBy()), FormatFloat(next)),
	}, nil
}

// ListenAndServe starts the gRPC server at listenURI.
func ListenAndServe(listenURI string, reflection bool) error {
	acc := &Accumulator{}
	return serve.RunWithOptions(listenURI, func(s *grpc.Server) {
		pb.RegisterCalculatorServiceServer(s, &Server{Acc: acc})
	}, reflection)
}
