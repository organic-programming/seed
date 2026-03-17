// Package server implements the GreetingService gRPC server.
package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/organic-programming/go-holons/pkg/serve"
	pb "github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/gen/go/greeting/v1"
	"github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/internal"

	"google.golang.org/grpc"
)

// Server implements the GreetingService.
type Server struct {
	pb.UnimplementedGreetingServiceServer
}

// ListLanguages returns all available greeting languages.
func (s *Server) ListLanguages(_ context.Context, _ *pb.ListLanguagesRequest) (*pb.ListLanguagesResponse, error) {
	langs := make([]*pb.Language, len(internal.Greetings))
	for i, g := range internal.Greetings {
		langs[i] = &pb.Language{
			Code:   g.Code,
			Name:   g.Name,
			Native: g.Native,
		}
	}
	return &pb.ListLanguagesResponse{Languages: langs}, nil
}

// SayHello greets the user in the requested language.
func (s *Server) SayHello(_ context.Context, req *pb.SayHelloRequest) (*pb.SayHelloResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "World"
	}
	g := internal.Lookup(req.LangCode)
	return &pb.SayHelloResponse{
		Greeting: fmt.Sprintf(g.Template, name),
		Language: g.Name,
		LangCode: g.Code,
	}, nil
}

// ListenAndServe starts the gRPC server at the given URI.
func ListenAndServe(listenURI string, reflection bool) error {
	return serve.RunWithOptions(listenURI, func(s *grpc.Server) {
		pb.RegisterGreetingServiceServer(s, &Server{})
	}, reflection)
}
