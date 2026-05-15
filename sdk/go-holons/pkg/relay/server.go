package relay

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	relayv1 "github.com/organic-programming/go-holons/gen/go/relay/v1"
	"github.com/organic-programming/go-holons/pkg/observability"
	"google.golang.org/grpc"
)

type RelayOptions struct {
	DownstreamConn *grpc.ClientConn
}

// RegisterServer registers the canonical RelayService on s.
func RegisterServer(s *grpc.Server, opts RelayOptions) {
	relayv1.RegisterRelayServiceServer(s, &server{downstream: opts.DownstreamConn})
}

type server struct {
	relayv1.UnimplementedRelayServiceServer
	downstream *grpc.ClientConn
	received   atomic.Int64
}

func (s *server) Tick(ctx context.Context, req *relayv1.TickRequest) (*relayv1.TickResponse, error) {
	count := s.received.Add(1)
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

	var hops []*relayv1.HopReceipt
	if s.downstream != nil {
		resp, err := relayv1.NewRelayServiceClient(s.downstream).Tick(ctx, req)
		if err != nil {
			return nil, err
		}
		hops = append(hops, resp.GetHops()...)
	}
	hops = append(hops, &relayv1.HopReceipt{
		Slug:     slug,
		Uid:      uid,
		Received: count,
	})
	return &relayv1.TickResponse{
		ResponderSlug:        slug,
		ResponderInstanceUid: uid,
		Hops:                 hops,
	}, nil
}

func responderSlug(obs *observability.Observability) string {
	if slug := strings.TrimSpace(obs.Slug()); slug != "" {
		return slug
	}
	if len(os.Args) == 0 {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
}
