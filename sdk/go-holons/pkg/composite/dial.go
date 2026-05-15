package composite

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/grpc"
)

// Dial opens a gRPC connection to an existing peer holon at the given
// address. Use this for peer-to-peer observation. For parent->child links,
// use SpawnMember.
//
// address must be a concrete endpoint:
//   - "tcp://host:port"
//   - "unix:///path/to/socket" or "unix://path"
//   - "host:port" (treated as tcp)
//
// Slug-based discovery is not handled here; resolve the slug first and pass
// the resulting address. Transitive observability is off by default. Pass
// WithTransitiveObservability(true) to pull the peer's Logs(follow=true) and
// Events(follow=true) streams into the caller's local rings. Closing the
// returned connection closes those streams and lets the relay goroutines exit.
// Because this function returns *grpc.ClientConn directly, it cannot intercept
// Close; callers that leak the connection keep the relay alive too.
func Dial(ctx context.Context, address string, opts ...DialOption) (*grpc.ClientConn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	conn, desc, err := dialAddress(ctx, address, 10*time.Second)
	if err != nil {
		return nil, err
	}

	dialOpts := applyDialOptions(opts)
	transitive := false
	if dialOpts.transitiveObservability != nil {
		transitive = *dialOpts.transitiveObservability
	}
	if !transitive {
		return conn, nil
	}

	identity, err := resolveRelayMemberIdentity(ctx, conn, desc)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := startRelayOn(context.Background(), identity.slug, identity.uid, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

type relayMemberIdentity struct {
	slug string
	uid  string
}

func resolveRelayMemberIdentity(ctx context.Context, conn *grpc.ClientConn, desc *holonsv1.DescribeResponse) (relayMemberIdentity, error) {
	baseSlug := slugFromDescribe(desc)
	resolveCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if identity, ok, _ := identityFromEvents(resolveCtx, conn, baseSlug); ok {
		return identity, nil
	}
	if identity, ok, err := identityFromLogs(resolveCtx, conn, baseSlug); ok {
		return identity, nil
	} else if err != nil {
		return relayMemberIdentity{}, fmt.Errorf("resolve relay identity: %w", err)
	}
	return relayMemberIdentity{}, fmt.Errorf("resolve relay identity: peer did not expose a local log or event with slug and instance_uid")
}

func identityFromEvents(ctx context.Context, conn *grpc.ClientConn, fallbackSlug string) (relayMemberIdentity, bool, error) {
	stream, err := holonsv1.NewHolonObservabilityClient(conn).Events(ctx, &holonsv1.EventsRequest{Follow: false})
	if err != nil {
		return relayMemberIdentity{}, false, err
	}
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return relayMemberIdentity{}, false, nil
		}
		if err != nil {
			return relayMemberIdentity{}, false, err
		}
		if len(event.GetChain()) > 0 || strings.TrimSpace(event.GetInstanceUid()) == "" {
			continue
		}
		slug := strings.TrimSpace(event.GetSlug())
		if slug == "" {
			slug = fallbackSlug
		}
		if slug == "" {
			continue
		}
		return relayMemberIdentity{slug: slug, uid: event.GetInstanceUid()}, true, nil
	}
}

func identityFromLogs(ctx context.Context, conn *grpc.ClientConn, fallbackSlug string) (relayMemberIdentity, bool, error) {
	stream, err := holonsv1.NewHolonObservabilityClient(conn).Logs(ctx, &holonsv1.LogsRequest{Follow: false})
	if err != nil {
		return relayMemberIdentity{}, false, err
	}
	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			return relayMemberIdentity{}, false, nil
		}
		if err != nil {
			return relayMemberIdentity{}, false, err
		}
		if len(entry.GetChain()) > 0 || strings.TrimSpace(entry.GetInstanceUid()) == "" {
			continue
		}
		slug := strings.TrimSpace(entry.GetSlug())
		if slug == "" {
			slug = fallbackSlug
		}
		if slug == "" {
			continue
		}
		return relayMemberIdentity{slug: slug, uid: entry.GetInstanceUid()}, true, nil
	}
}

func slugFromDescribe(desc *holonsv1.DescribeResponse) string {
	identity := desc.GetManifest().GetIdentity()
	for _, alias := range identity.GetAliases() {
		if trimmed := strings.TrimSpace(alias); trimmed != "" {
			return trimmed
		}
	}
	if slug := slugify(identity.GetGivenName() + "-" + identity.GetFamilyName()); slug != "" {
		return slug
	}
	return slugify(identity.GetFamilyName())
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
