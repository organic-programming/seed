package server

import (
	"context"

	pb "github.com/organic-programming/seed/recipes/composition/workers/charon-worker-transform/gen/go/transform/v1"
	"github.com/organic-programming/seed/recipes/composition/workers/charon-worker-transform/internal"
)

// Server implements transform.v1.TransformService.
type Server struct {
	pb.UnimplementedTransformServiceServer
}

// Transform returns the reversed input string.
func (s *Server) Transform(_ context.Context, req *pb.TransformRequest) (*pb.TransformResponse, error) {
	return &pb.TransformResponse{Result: internal.Reverse(req.GetText())}, nil
}
