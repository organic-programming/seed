package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/organic-programming/go-holons/pkg/observability"
	"github.com/organic-programming/go-holons/pkg/serve"

	pb "observability-cascade-node-go/gen/go/relay/v1"

	"google.golang.org/grpc"
)

// Server implements RelayService.
type Server struct {
	pb.UnimplementedRelayServiceServer
}

// Tick emits the minimal business signal used by observability-cascade.
func (s *Server) Tick(ctx context.Context, req *pb.TickRequest) (*pb.TickResponse, error) {
	obs := observability.Current()
	slug := responderSlug(obs)
	uid := obs.InstanceUID()
	obs.Logger("tick").InfoContext(ctx, "tick received",
		"sender", req.GetSender(),
		"note", req.GetNote(),
		"responder_slug", slug,
		"responder_uid", uid,
	)
	obs.Counter("cascade_ticks_total", "Ticks received by this cascade node.", map[string]string{
		"responder_uid": uid,
	}).Inc()
	return &pb.TickResponse{
		ResponderSlug:        slug,
		ResponderInstanceUid: uid,
	}, nil
}

// ListenAndServe starts the gRPC server at listenURI.
func ListenAndServe(listenURI string, reflection bool, members []serve.MemberRef, moreListenURIs ...string) error {
	options := serve.ServeOptions{
		Reflect:         reflection,
		MemberEndpoints: members,
	}
	return serve.RunWithServeOptions(listenURI, func(s *grpc.Server) {
		pb.RegisterRelayServiceServer(s, &Server{})
	}, options, moreListenURIs...)
}

func responderSlug(obs *observability.Observability) string {
	if slug := strings.TrimSpace(obs.Slug()); slug != "" {
		return slug
	}
	return strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
}
