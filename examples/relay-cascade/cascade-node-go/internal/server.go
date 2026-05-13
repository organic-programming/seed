package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/organic-programming/go-holons/pkg/observability"
	"github.com/organic-programming/go-holons/pkg/serve"

	pb "cascade-node-go/gen/go/relay/v1"

	"google.golang.org/grpc"
)

// Server implements RelayService.
type Server struct {
	pb.UnimplementedRelayServiceServer
}

// Tick emits one minimal tick response. Observability is wired in step 2.
func (s *Server) Tick(_ context.Context, _ *pb.TickRequest) (*pb.TickResponse, error) {
	obs := observability.Current()
	return &pb.TickResponse{
		ResponderSlug:        responderSlug(obs),
		ResponderInstanceUid: obs.InstanceUID(),
	}, nil
}

// ListenAndServe starts the gRPC server at listenURI.
func ListenAndServe(listenURI string, reflection bool) error {
	return serve.RunWithOptions(listenURI, func(s *grpc.Server) {
		pb.RegisterRelayServiceServer(s, &Server{})
	}, reflection)
}

func responderSlug(obs *observability.Observability) string {
	if slug := strings.TrimSpace(obs.Slug()); slug != "" {
		return slug
	}
	return strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
}
