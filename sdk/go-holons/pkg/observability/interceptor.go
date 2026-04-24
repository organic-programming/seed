package observability

import (
	"context"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// SessionIDMetadataKey is the gRPC metadata header that optionally
// carries a pre-existing session identifier. When not set, the
// interceptor generates a per-call identifier.
const SessionIDMetadataKey = "x-holon-session-id"

// UnaryServerInterceptor returns a gRPC interceptor that:
//
//   - Injects `rpc_method` and `session_id` into the context so
//     Loggers emit them as well-known fields.
//   - Emits baseline metrics under the `holon_handler_*` and
//     `holon_session_rpc_*` namespaces.
//   - Recovers handler panics, counts them, and emits a HANDLER_PANIC
//     event before rethrowing so higher-level recovery can still run.
//
// The interceptor is a no-op when Observability is disabled.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		obs := Current()
		method := info.FullMethod
		sessionID := sessionIDFromMetadata(ctx)
		if sessionID == "" {
			sessionID = generateSessionID(method)
		}

		// Attach to context for logger correlation.
		ctx = WithContext(ctx, CtxValues{
			SessionID: sessionID,
			RPCMethod: method,
		})

		// Baseline: in-flight gauge.
		inflight := obs.Gauge("holon_handler_in_flight",
			"Currently executing RPCs per method.",
			map[string]string{"method": method})
		inflight.Add(1)
		defer inflight.Add(-1)

		// Panic recovery + HANDLER_PANIC event.
		defer func() {
			if r := recover(); r != nil {
				obs.Counter("holon_handler_panics_total",
					"Recovered handler panics per method.",
					map[string]string{"method": method}).Inc()
				obs.Emit(ctx, EventHandlerPanic, map[string]string{
					"method": method,
					"panic":  stringify(r),
				})
				// Re-panic so downstream recovery (if any) still runs.
				panic(r)
			}
		}()

		start := time.Now()
		resp, err = handler(ctx, req)
		elapsed := time.Since(start)

		// RPC count + duration. v1 reports only phase=total; the
		// four-phase decomposition belongs to the v2 session metrics store.
		obs.Counter("holon_session_rpc_total",
			"Session RPC count by method, direction, phase.",
			map[string]string{
				"method":    method,
				"direction": "inbound",
				"phase":     "total",
			}).Inc()

		obs.Histogram("holon_session_rpc_duration_seconds",
			"Session RPC duration decomposed into wire_out/queue/work/wire_in/total.",
			map[string]string{
				"method":    method,
				"direction": "inbound",
				"phase":     "total",
			},
			nil).ObserveDuration(elapsed)

		return resp, err
	}
}

func sessionIDFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vs := md.Get(SessionIDMetadataKey)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// generateSessionID returns a best-effort unique id for an RPC when no
// client-provided session id is available. Uses time-based monotonicity
// plus a package-scope atomic counter so concurrent handlers never
// collide; adequate for log correlation, not guaranteed to be globally
// unique across processes.
var sessionCounter atomic.Uint64

func generateSessionID(method string) string {
	n := time.Now().UnixNano()
	_ = method
	return formatID(n, sessionCounter.Add(1))
}

func formatID(n int64, ctr uint64) string {
	// Minimal base-36 encoding without strconv hotness.
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [20]byte
	i := len(buf)
	v := uint64(n) ^ (ctr << 17)
	for v > 0 {
		i--
		buf[i] = alphabet[v%36]
		v /= 36
	}
	if i == len(buf) {
		return "0"
	}
	return string(buf[i:])
}
