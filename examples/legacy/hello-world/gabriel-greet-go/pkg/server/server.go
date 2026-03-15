// Package server implements the HelloService gRPC server.
package server

import (
	"context"
	"fmt"

	pb "github.com/organic-programming/examples/hello-world/gabriel-greet-go/gen/go/hello/v1"
	"github.com/organic-programming/go-holons/pkg/serve"

	"google.golang.org/grpc"
)

// Server implements the HelloService.
type Server struct {
	pb.UnimplementedHelloServiceServer
}

// Greet returns a greeting for the given name.
func (s *Server) Greet(_ context.Context, req *pb.GreetRequest) (*pb.GreetResponse, error) {
	name := req.Name
	if name == "" {
		name = "World"
	}
	return &pb.GreetResponse{
		Message: fmt.Sprintf("Hello %s", name),
	}, nil
}

// ListenAndServe starts the gRPC server at the given URI.
func ListenAndServe(listenURI string, reflection bool) error {
	return serve.RunWithOptions(listenURI, func(s *grpc.Server) {
		pb.RegisterHelloServiceServer(s, &Server{})
	}, reflection)
}
