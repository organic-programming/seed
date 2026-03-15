// Package internal contains the greeting data and server implementation.
package internal

import (
	"context"
	"fmt"
	"strings"

	"github.com/organic-programming/go-holons/pkg/serve"

	pb "gabriel-greeting-go/gen/go/greeting/v1"

	"google.golang.org/grpc"
)

// Server implements the GreetingService.
type Server struct {
	pb.UnimplementedGreetingServiceServer
}

// ListLanguages returns all available greeting languages.
func (s *Server) ListLanguages(_ context.Context, _ *pb.ListLanguagesRequest) (*pb.ListLanguagesResponse, error) {
	langs := make([]*pb.Language, len(Greetings))
	for i, g := range Greetings {
		langs[i] = &pb.Language{
			Code:   g.LangCode,
			Name:   g.LangEnglish,
			Native: g.LangNative,
		}
	}
	return &pb.ListLanguagesResponse{Languages: langs}, nil
}

// SayHello greets the user in the requested language.
func (s *Server) SayHello(_ context.Context, req *pb.SayHelloRequest) (*pb.SayHelloResponse, error) {
	g := Lookup(req.LangCode)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = g.DefaultName
	}
	return &pb.SayHelloResponse{
		Greeting: fmt.Sprintf(g.Template, name),
		Language: g.LangEnglish,
		LangCode: g.LangCode,
	}, nil
}

// ListenAndServe starts the gRPC server at the given URI.
func ListenAndServe(listenURI string, reflection bool) error {
	return serve.RunWithOptions(listenURI, func(s *grpc.Server) {
		pb.RegisterGreetingServiceServer(s, &Server{})
	}, reflection)
}
