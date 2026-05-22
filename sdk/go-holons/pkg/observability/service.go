package observability

import (
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Visibility mirrors the ObservabilityVisibility enum from
// holons/v1/manifest.proto.
type Visibility int32

const (
	VisibilityUnspecified Visibility = 0
	VisibilityOff         Visibility = 1
	VisibilitySummary     Visibility = 2
	VisibilityFull        Visibility = 3
)

type service struct {
	v1.UnimplementedHolonObservabilityServer
	obs *Observability
	vis Visibility
}

func NewService(obs *Observability, vis Visibility) v1.HolonObservabilityServer {
	if vis == VisibilityUnspecified {
		vis = VisibilityFull
	}
	return &service{obs: obs, vis: vis}
}

func (s *service) Logs(req *v1.LogsRequest, stream v1.HolonObservability_LogsServer) error {
	if s.vis == VisibilityOff {
		return status.Error(codes.PermissionDenied, "observability visibility is OFF")
	}
	if s.obs == nil || !s.obs.Enabled(FamilyLogs) {
		return status.Error(codes.FailedPrecondition, "logs family is not enabled (OP_OBS)")
	}

	minLevel := Level(req.MinSeverityNumber)
	if minLevel == LevelUnset {
		minLevel = LevelInfo
	}
	sessionFilter := toSet(req.SessionIds)
	methodFilter := toSet(req.RpcMethods)

	var cutoff time.Time
	if req.Since != nil {
		cutoff = time.Now().Add(-req.Since.AsDuration())
	}
	ring := s.obs.LogRing()
	var replay []LogRecord
	var ch <-chan LogRecord
	var stop func()
	if req.Follow {
		replay, ch, stop = ring.replayAndWatch(cutoff, 128)
		defer stop()
	} else if !cutoff.IsZero() {
		replay = ring.DrainSince(cutoff)
	} else {
		replay = ring.Drain()
	}
	for _, e := range replay {
		if e.Private || !s.matchLog(e, minLevel, sessionFilter, methodFilter) {
			continue
		}
		if err := stream.Send(s.maskLog(ToProtoLogRecord(e))); err != nil {
			return err
		}
	}
	if !req.Follow {
		return nil
	}

	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if e.Private || !s.matchLog(e, minLevel, sessionFilter, methodFilter) {
				continue
			}
			if err := stream.Send(s.maskLog(ToProtoLogRecord(e))); err != nil {
				return err
			}
		}
	}
}

func (s *service) matchLog(e LogRecord, minLevel Level, sessionFilter, methodFilter map[string]struct{}) bool {
	if e.Record == nil || Level(e.Record.GetSeverityNumber()) < minLevel {
		return false
	}
	if len(sessionFilter) > 0 {
		if _, ok := sessionFilter[e.attr(AttrHolonsSessionID)]; !ok {
			return false
		}
	}
	if len(methodFilter) > 0 {
		if _, ok := methodFilter[e.attr(AttrRPCMethod)]; !ok {
			return false
		}
	}
	return true
}

func (s *service) maskLog(p *v1.LogRecord) *v1.LogRecord {
	if s.vis != VisibilitySummary {
		return p
	}
	return &v1.LogRecord{
		TimeUnixNano:         p.GetTimeUnixNano(),
		ObservedTimeUnixNano: p.GetObservedTimeUnixNano(),
		SeverityNumber:       p.GetSeverityNumber(),
		SeverityText:         p.GetSeverityText(),
		Attributes:           resourceAttributes(StringAttribute(p.GetAttributes(), AttrHolonsSlug), ""),
		Chain:                CloneChain(p.GetChain()),
	}
}

func (s *service) Metrics(req *v1.MetricsRequest, stream v1.HolonObservability_MetricsServer) error {
	if s.vis == VisibilityOff {
		return status.Error(codes.PermissionDenied, "observability visibility is OFF")
	}
	if s.obs == nil || !s.obs.Enabled(FamilyMetrics) {
		return status.Error(codes.FailedPrecondition, "metrics family is not enabled (OP_OBS)")
	}

	snap := s.obs.Registry().Snapshot()
	metrics := ToProtoMetrics(snap, s.obs.Slug(), s.obs.InstanceUID(), s.obs.startWall)
	if len(req.NamePrefixes) > 0 {
		metrics = filterMetricsByPrefix(metrics, req.NamePrefixes)
	}
	if s.vis == VisibilitySummary {
		for _, metric := range metrics {
			maskMetric(metric)
		}
	}
	for _, metric := range metrics {
		if err := stream.Send(metric); err != nil {
			return err
		}
	}
	return nil
}

func filterMetricsByPrefix(src []*v1.Metric, prefixes []string) []*v1.Metric {
	out := src[:0]
	for _, metric := range src {
		for _, p := range prefixes {
			if p == "" {
				continue
			}
			if len(metric.Name) >= len(p) && metric.Name[:len(p)] == p {
				out = append(out, metric)
				break
			}
		}
	}
	cpy := make([]*v1.Metric, len(out))
	copy(cpy, out)
	return cpy
}

func maskMetric(metric *v1.Metric) {
	switch data := metric.GetData().(type) {
	case *v1.Metric_Gauge:
		for _, dp := range data.Gauge.GetDataPoints() {
			dp.Value = &v1.NumberDataPoint_AsDouble{AsDouble: 0}
			dp.Attributes = resourceAttributes(StringAttribute(dp.GetAttributes(), AttrHolonsSlug), "")
		}
	case *v1.Metric_Sum:
		for _, dp := range data.Sum.GetDataPoints() {
			dp.Value = &v1.NumberDataPoint_AsInt{AsInt: 0}
			dp.Attributes = resourceAttributes(StringAttribute(dp.GetAttributes(), AttrHolonsSlug), "")
		}
	case *v1.Metric_Histogram:
		for _, dp := range data.Histogram.GetDataPoints() {
			dp.Count = 0
			dp.Sum = 0
			dp.BucketCounts = nil
			dp.ExplicitBounds = nil
			dp.Attributes = resourceAttributes(StringAttribute(dp.GetAttributes(), AttrHolonsSlug), "")
		}
	}
}

func (s *service) Events(req *v1.EventsRequest, stream v1.HolonObservability_EventsServer) error {
	if s.vis == VisibilityOff {
		return status.Error(codes.PermissionDenied, "observability visibility is OFF")
	}
	if s.obs == nil || !s.obs.Enabled(FamilyEvents) {
		return status.Error(codes.FailedPrecondition, "events family is not enabled (OP_OBS)")
	}

	names := toSet(req.EventNames)
	var cutoff time.Time
	if req.Since != nil {
		cutoff = time.Now().Add(-req.Since.AsDuration())
	}
	bus := s.obs.EventBus()
	var replay []LogRecord
	var ch <-chan LogRecord
	var stop func()
	if req.Follow {
		replay, ch, stop = bus.replayAndWatch(cutoff, 64)
		defer stop()
	} else if !cutoff.IsZero() {
		replay = bus.DrainSince(cutoff)
	} else {
		replay = bus.Drain()
	}
	for _, e := range replay {
		if e.Private || !matchEventName(e, names) {
			continue
		}
		if err := stream.Send(s.maskEvent(ToProtoLogRecord(e))); err != nil {
			return err
		}
	}
	if !req.Follow {
		return nil
	}

	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if e.Private || !matchEventName(e, names) {
				continue
			}
			if err := stream.Send(s.maskEvent(ToProtoLogRecord(e))); err != nil {
				return err
			}
		}
	}
}

func (s *service) maskEvent(p *v1.LogRecord) *v1.LogRecord {
	if s.vis != VisibilitySummary {
		return p
	}
	return &v1.LogRecord{
		TimeUnixNano:         p.GetTimeUnixNano(),
		ObservedTimeUnixNano: p.GetObservedTimeUnixNano(),
		EventName:            p.GetEventName(),
		Attributes:           resourceAttributes(StringAttribute(p.GetAttributes(), AttrHolonsSlug), ""),
		Chain:                CloneChain(p.GetChain()),
	}
}

func matchEventName(record LogRecord, filter map[string]struct{}) bool {
	if len(filter) == 0 {
		return true
	}
	if record.Record == nil {
		return false
	}
	_, ok := filter[record.Record.GetEventName()]
	return ok
}

func init() {
	_ = toSet
}
