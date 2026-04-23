package observability

import (
	"math"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ToProtoLogEntry converts an in-memory LogEntry into its proto form.
// The returned message is safe to send on the wire.
func ToProtoLogEntry(e LogEntry) *v1.LogEntry {
	out := &v1.LogEntry{
		Ts:          timestamppb.New(e.Timestamp),
		Level:       v1.LogLevel(e.Level),
		Slug:        e.Slug,
		InstanceUid: e.InstanceUID,
		SessionId:   e.SessionID,
		RpcMethod:   e.RPCMethod,
		Message:     e.Message,
		Fields:      e.Fields,
		Caller:      e.Caller,
	}
	if len(e.Chain) > 0 {
		out.Chain = make([]*v1.ChainHop, len(e.Chain))
		for i, h := range e.Chain {
			out.Chain[i] = &v1.ChainHop{Slug: h.Slug, InstanceUid: h.InstanceUID}
		}
	}
	return out
}

// FromProtoLogEntry converts a proto LogEntry into its in-memory form.
func FromProtoLogEntry(p *v1.LogEntry) LogEntry {
	if p == nil {
		return LogEntry{}
	}
	out := LogEntry{
		Level:       Level(p.Level),
		Slug:        p.Slug,
		InstanceUID: p.InstanceUid,
		SessionID:   p.SessionId,
		RPCMethod:   p.RpcMethod,
		Message:     p.Message,
		Fields:      p.Fields,
		Caller:      p.Caller,
	}
	if p.Ts != nil {
		out.Timestamp = p.Ts.AsTime()
	}
	if len(p.Chain) > 0 {
		out.Chain = make([]Hop, len(p.Chain))
		for i, h := range p.Chain {
			out.Chain[i] = Hop{Slug: h.Slug, InstanceUID: h.InstanceUid}
		}
	}
	return out
}

// ToProtoEvent converts an in-memory Event into its proto form.
func ToProtoEvent(e Event) *v1.EventInfo {
	out := &v1.EventInfo{
		Ts:          timestamppb.New(e.Timestamp),
		Type:        v1.EventType(e.Type),
		Slug:        e.Slug,
		InstanceUid: e.InstanceUID,
		SessionId:   e.SessionID,
		Payload:     e.Payload,
	}
	if len(e.Chain) > 0 {
		out.Chain = make([]*v1.ChainHop, len(e.Chain))
		for i, h := range e.Chain {
			out.Chain[i] = &v1.ChainHop{Slug: h.Slug, InstanceUid: h.InstanceUID}
		}
	}
	return out
}

// FromProtoEvent converts a proto EventInfo back into an in-memory Event.
func FromProtoEvent(p *v1.EventInfo) Event {
	if p == nil {
		return Event{}
	}
	out := Event{
		Type:        EventType(p.Type),
		Slug:        p.Slug,
		InstanceUID: p.InstanceUid,
		SessionID:   p.SessionId,
		Payload:     p.Payload,
	}
	if p.Ts != nil {
		out.Timestamp = p.Ts.AsTime()
	}
	if len(p.Chain) > 0 {
		out.Chain = make([]Hop, len(p.Chain))
		for i, h := range p.Chain {
			out.Chain[i] = Hop{Slug: h.Slug, InstanceUID: h.InstanceUid}
		}
	}
	return out
}

// ToProtoMetricSamples walks a RegistrySnapshot and emits one
// MetricSample per counter/gauge/histogram.
func ToProtoMetricSamples(snap RegistrySnapshot) []*v1.MetricSample {
	out := make([]*v1.MetricSample, 0, len(snap.Counters)+len(snap.Gauges)+len(snap.Histograms))
	for _, c := range snap.Counters {
		out = append(out, &v1.MetricSample{
			Name:   c.Name,
			Help:   c.Help,
			Labels: c.Labels,
			Value:  &v1.MetricSample_Counter{Counter: c.Value},
		})
	}
	for _, g := range snap.Gauges {
		out = append(out, &v1.MetricSample{
			Name:   g.Name,
			Help:   g.Help,
			Labels: g.Labels,
			Value:  &v1.MetricSample_Gauge{Gauge: g.Value},
		})
	}
	for _, h := range snap.Histograms {
		hp := &v1.HistogramSample{
			Count: h.Snap.Total,
			Sum:   h.Snap.Sum,
		}
		for i := range h.Snap.Bounds {
			hp.Buckets = append(hp.Buckets, &v1.Bucket{
				UpperBound: h.Snap.Bounds[i],
				Count:      h.Snap.Counts[i],
			})
		}
		out = append(out, &v1.MetricSample{
			Name:   h.Name,
			Help:   h.Help,
			Labels: h.Labels,
			Value:  &v1.MetricSample_Histogram{Histogram: hp},
		})
	}
	return out
}

// histogramInfPlus returns the +Inf upper bound in a form protoc-gen-go
// accepts for JSON/proto emission. Kept out-of-line for test access.
func histogramInfPlus() float64 { return math.Inf(1) }
