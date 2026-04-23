package observability

import (
	"context"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/grpc"
)

// Register attaches the HolonObservability service to the supplied
// gRPC server using the active Observability singleton. No-op when
// Observability is disabled (none of the families in OP_OBS).
//
// The caller is responsible for installing UnaryServerInterceptor()
// as part of the grpc.NewServer options — Register does not alter the
// server's interceptor chain.
func Register(s *grpc.Server) {
	obs := Current()
	if obs == nil || (!obs.Enabled(FamilyLogs) && !obs.Enabled(FamilyMetrics) && !obs.Enabled(FamilyEvents)) {
		return
	}
	v1.RegisterHolonObservabilityServer(s, NewService(obs, VisibilityFull))
}

// EmitReady publishes an INSTANCE_READY event through the active
// Observability. Serve runners should call this immediately after the
// first listener binds.
func EmitReady(ctx context.Context, listenerURI string) {
	Current().Emit(ctx, EventInstanceReady, map[string]string{
		"listener": listenerURI,
	})
}

// EmitExited publishes an INSTANCE_EXITED event with the given exit
// reason. Serve runners should call this during graceful shutdown.
func EmitExited(ctx context.Context, reason string) {
	Current().Emit(ctx, EventInstanceExited, map[string]string{
		"reason": reason,
	})
}

// EmitCrashed publishes an INSTANCE_CRASHED event with the given
// exit code / cause. Use during panic recovery or when a fatal
// condition prevents clean exit.
func EmitCrashed(ctx context.Context, cause string) {
	Current().Emit(ctx, EventInstanceCrashed, map[string]string{
		"cause": cause,
	})
}
