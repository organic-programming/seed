package server

import (
	"context"

	pb "github.com/organic-programming/seed/recipes/composition/workers/charon-worker-compute/gen/go/compute/v1"
	"github.com/organic-programming/seed/recipes/composition/workers/charon-worker-compute/internal"
)

// Server implements compute.v1.ComputeService.
type Server struct {
	pb.UnimplementedComputeServiceServer
}

// Compute returns the square of the input value.
func (s *Server) Compute(_ context.Context, req *pb.ComputeRequest) (*pb.ComputeResponse, error) {
	return &pb.ComputeResponse{Result: internal.Square(req.GetValue())}, nil
}
