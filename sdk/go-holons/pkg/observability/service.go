package observability

import (
	"context"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Visibility mirrors the ObservabilityVisibility enum from
// holons/v1/manifest.proto. The SDK keeps its own typed alias so this
// package does not have to import the v1 proto for every reference.
type Visibility int32

const (
	VisibilityUnspecified Visibility = 0
	VisibilityOff         Visibility = 1
	VisibilitySummary     Visibility = 2
	VisibilityFull        Visibility = 3
)

// service is the gRPC HolonObservability implementation backed by the
// active Observability singleton. A single service instance serves
// every inbound call; per-listener visibility is applied at call time
// from the Observability.cfg or a future per-stream lookup. See
// OBSERVABILITY.md §Security Considerations.
type service struct {
	v1.UnimplementedHolonObservabilityServer
	obs *Observability
	vis Visibility
}

// NewService constructs the gRPC implementation. The visibility argument
// is the default applied when the per-listener override does not match
// (v1 is a single global dial; per-listener overrides will arrive when
// the serve runner exposes listener identity through context).
func NewService(obs *Observability, vis Visibility) v1.HolonObservabilityServer {
	if vis == VisibilityUnspecified {
		vis = VisibilityFull
	}
	return &service{obs: obs, vis: vis}
}

// Logs streams log entries. Summary mode redacts to ts+level+slug.
// Off returns PERMISSION_DENIED.
func (s *service) Logs(req *v1.LogsRequest, stream v1.HolonObservability_LogsServer) error {
	if s.vis == VisibilityOff {
		return status.Error(codes.PermissionDenied, "observability visibility is OFF")
	}
	if s.obs == nil || !s.obs.Enabled(FamilyLogs) {
		return status.Error(codes.FailedPrecondition, "logs family is not enabled (OP_OBS)")
	}

	minLevel := Level(req.MinLevel)
	if minLevel == LevelUnset {
		minLevel = LevelInfo
	}
	sessionFilter := toSet(req.SessionIds)
	methodFilter := toSet(req.RpcMethods)

	// Replay phase.
	var cutoff time.Time
	if req.Since != nil {
		cutoff = time.Now().Add(-req.Since.AsDuration())
	}
	ring := s.obs.LogRing()
	var replay []LogEntry
	if !cutoff.IsZero() {
		replay = ring.DrainSince(cutoff)
	} else {
		replay = ring.Drain()
	}
	for _, e := range replay {
		if !s.matchLog(e, minLevel, sessionFilter, methodFilter) {
			continue
		}
		if err := stream.Send(s.maskLog(ToProtoLogEntry(e))); err != nil {
			return err
		}
	}
	if !req.Follow {
		return nil
	}

	// Live phase.
	ctx := stream.Context()
	ch, stop := ring.Watch(128)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if !s.matchLog(e, minLevel, sessionFilter, methodFilter) {
				continue
			}
			if err := stream.Send(s.maskLog(ToProtoLogEntry(e))); err != nil {
				return err
			}
		}
	}
}

func (s *service) matchLog(e LogEntry, minLevel Level, sessionFilter, methodFilter map[string]struct{}) bool {
	if e.Level < minLevel {
		return false
	}
	if len(sessionFilter) > 0 {
		if _, ok := sessionFilter[e.SessionID]; !ok {
			return false
		}
	}
	if len(methodFilter) > 0 {
		if _, ok := methodFilter[e.RPCMethod]; !ok {
			return false
		}
	}
	return true
}

// maskLog applies the Summary reduction: when visibility is SUMMARY
// the handler strips every field except ts, level and slug.
func (s *service) maskLog(p *v1.LogEntry) *v1.LogEntry {
	if s.vis != VisibilitySummary {
		return p
	}
	return &v1.LogEntry{
		Ts:    p.Ts,
		Level: p.Level,
		Slug:  p.Slug,
	}
}

// Metrics returns a point-in-time snapshot of the registry.
func (s *service) Metrics(ctx context.Context, req *v1.MetricsRequest) (*v1.MetricsSnapshot, error) {
	if s.vis == VisibilityOff {
		return nil, status.Error(codes.PermissionDenied, "observability visibility is OFF")
	}
	if s.obs == nil || !s.obs.Enabled(FamilyMetrics) {
		return nil, status.Error(codes.FailedPrecondition, "metrics family is not enabled (OP_OBS)")
	}

	snap := s.obs.Registry().Snapshot()
	samples := ToProtoMetricSamples(snap)
	if len(req.NamePrefixes) > 0 {
		samples = filterByPrefix(samples, req.NamePrefixes)
	}
	if s.vis == VisibilitySummary {
		// Summary: metric names + counts only. Values reduced to zero.
		for _, ms := range samples {
			switch ms.Value.(type) {
			case *v1.MetricSample_Counter:
				ms.Value = &v1.MetricSample_Counter{Counter: 0}
			case *v1.MetricSample_Gauge:
				ms.Value = &v1.MetricSample_Gauge{Gauge: 0}
			case *v1.MetricSample_Histogram:
				ms.Value = &v1.MetricSample_Histogram{Histogram: &v1.HistogramSample{}}
			}
			ms.Labels = nil
		}
	}
	return &v1.MetricsSnapshot{
		CapturedAt:  timestamppb.New(snap.CapturedAt),
		Slug:        s.obs.Slug(),
		InstanceUid: s.obs.InstanceUID(),
		Samples:     samples,
		// SessionRollup is populated in P5 when the session metrics
		// store is wired into observability.
	}, nil
}

func filterByPrefix(src []*v1.MetricSample, prefixes []string) []*v1.MetricSample {
	out := src[:0]
	for _, s := range src {
		for _, p := range prefixes {
			if len(p) == 0 {
				continue
			}
			if len(s.Name) >= len(p) && s.Name[:len(p)] == p {
				out = append(out, s)
				break
			}
		}
	}
	cpy := make([]*v1.MetricSample, len(out))
	copy(cpy, out)
	return cpy
}

// Events streams lifecycle events.
func (s *service) Events(req *v1.EventsRequest, stream v1.HolonObservability_EventsServer) error {
	if s.vis == VisibilityOff {
		return status.Error(codes.PermissionDenied, "observability visibility is OFF")
	}
	if s.obs == nil || !s.obs.Enabled(FamilyEvents) {
		return status.Error(codes.FailedPrecondition, "events family is not enabled (OP_OBS)")
	}

	types := typeSet(req.Types)

	var cutoff time.Time
	if req.Since != nil {
		cutoff = time.Now().Add(-req.Since.AsDuration())
	}
	bus := s.obs.EventBus()
	var replay []Event
	if !cutoff.IsZero() {
		replay = bus.DrainSince(cutoff)
	} else {
		replay = bus.Drain()
	}
	for _, e := range replay {
		if !matchEventType(e.Type, types) {
			continue
		}
		if err := stream.Send(s.maskEvent(ToProtoEvent(e))); err != nil {
			return err
		}
	}
	if !req.Follow {
		return nil
	}

	ctx := stream.Context()
	ch, stop := bus.Watch(64)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if !matchEventType(e.Type, types) {
				continue
			}
			if err := stream.Send(s.maskEvent(ToProtoEvent(e))); err != nil {
				return err
			}
		}
	}
}

func (s *service) maskEvent(p *v1.EventInfo) *v1.EventInfo {
	if s.vis != VisibilitySummary {
		return p
	}
	return &v1.EventInfo{
		Ts:   p.Ts,
		Type: p.Type,
	}
}

func typeSet(in []v1.EventType) map[v1.EventType]struct{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[v1.EventType]struct{}, len(in))
	for _, t := range in {
		out[t] = struct{}{}
	}
	return out
}

func matchEventType(t EventType, filter map[v1.EventType]struct{}) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[v1.EventType(t)]
	return ok
}

// internal helper — strings.TrimSpace rooted filter set builder.
// Lives in observability.go's toSet but duplicated here to keep the
// proto module independent. The cost is one extra declaration.
func init() {
	_ = toSet // reference to silence unused-function warnings during
	// partial build states; removed by linker.
}
