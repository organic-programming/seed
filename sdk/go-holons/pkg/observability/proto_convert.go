package observability

import (
	"fmt"
	"math"
	"sort"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/protobuf/proto"
)

const (
	AttrHolonsSlug        = "holons.slug"
	AttrHolonsInstanceUID = "holons.instance_uid"
	AttrHolonsSessionID   = "holons.session_id"
	AttrHolonsTransport   = "holons.transport"
	AttrServiceName       = "service.name"
	AttrServiceInstanceID = "service.instance.id"
	AttrRPCMethod         = "rpc.method"
	AttrLoggerName        = "logger.name"
	AttrCodeCaller        = "code.caller"
)

// LogRecord is the SDK-local envelope for an OTLP-shaped wire record.
// Private is local-only and is never sent through HolonObservability.
type LogRecord struct {
	Record  *v1.LogRecord
	Private bool
}

func newLogRecord(record *v1.LogRecord, private bool) LogRecord {
	if record == nil {
		record = &v1.LogRecord{}
	}
	return LogRecord{Record: record, Private: private}
}

// ToProtoLogRecord converts an in-memory LogRecord into its wire form.
func ToProtoLogRecord(r LogRecord) *v1.LogRecord {
	return cloneLogRecord(r.Record)
}

// FromProtoLogRecord converts a wire LogRecord into the SDK-local envelope.
func FromProtoLogRecord(p *v1.LogRecord) LogRecord {
	return LogRecord{Record: cloneLogRecord(p)}
}

func cloneLogRecord(p *v1.LogRecord) *v1.LogRecord {
	if p == nil {
		return &v1.LogRecord{}
	}
	return proto.Clone(p).(*v1.LogRecord)
}

func (r LogRecord) timestamp() time.Time {
	if r.Record == nil || r.Record.TimeUnixNano == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64(r.Record.TimeUnixNano))
}

func (r LogRecord) bodyString() string {
	if r.Record == nil {
		return ""
	}
	return anyValueString(r.Record.Body)
}

func (r LogRecord) attr(key string) string {
	if r.Record == nil {
		return ""
	}
	return StringAttribute(r.Record.Attributes, key)
}

// StringAttribute returns the string rendering of a KeyValue attribute.
func StringAttribute(attrs []*v1.KeyValue, key string) string {
	for _, attr := range attrs {
		if attr != nil && attr.Key == key {
			return anyValueString(attr.Value)
		}
	}
	return ""
}

func anyValueString(v *v1.AnyValue) string {
	if v == nil {
		return ""
	}
	switch x := v.Value.(type) {
	case *v1.AnyValue_StringValue:
		return x.StringValue
	case *v1.AnyValue_BoolValue:
		return fmt.Sprintf("%t", x.BoolValue)
	case *v1.AnyValue_IntValue:
		return fmt.Sprintf("%d", x.IntValue)
	case *v1.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", x.DoubleValue)
	default:
		return ""
	}
}

func severityLabel(n v1.SeverityNumber) string {
	switch n {
	case v1.SeverityNumber_SEVERITY_NUMBER_TRACE:
		return "TRACE"
	case v1.SeverityNumber_SEVERITY_NUMBER_DEBUG:
		return "DEBUG"
	case v1.SeverityNumber_SEVERITY_NUMBER_INFO:
		return "INFO"
	case v1.SeverityNumber_SEVERITY_NUMBER_WARN:
		return "WARN"
	case v1.SeverityNumber_SEVERITY_NUMBER_ERROR:
		return "ERROR"
	case v1.SeverityNumber_SEVERITY_NUMBER_FATAL:
		return "FATAL"
	default:
		return "UNSPECIFIED"
	}
}

func userAttributesMap(attrs []*v1.KeyValue) map[string]string {
	out := map[string]string{}
	for _, attr := range attrs {
		if attr == nil || isSystemAttribute(attr.Key) {
			continue
		}
		out[attr.Key] = anyValueString(attr.Value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isSystemAttribute(key string) bool {
	switch key {
	case AttrHolonsSlug, AttrHolonsInstanceUID, AttrHolonsSessionID, AttrHolonsTransport,
		AttrServiceName, AttrServiceInstanceID, AttrRPCMethod, AttrLoggerName, AttrCodeCaller:
		return true
	default:
		return false
	}
}

func ToAnyValue(v any) *v1.AnyValue {
	switch x := v.(type) {
	case nil:
		return &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: ""}}
	case string:
		return &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: x}}
	case bool:
		return &v1.AnyValue{Value: &v1.AnyValue_BoolValue{BoolValue: x}}
	case int:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: int64(x)}}
	case int8:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: int64(x)}}
	case int16:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: int64(x)}}
	case int32:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: int64(x)}}
	case int64:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: x}}
	case uint:
		return uintAnyValue(uint64(x))
	case uint8:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: int64(x)}}
	case uint16:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: int64(x)}}
	case uint32:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: int64(x)}}
	case uint64:
		return uintAnyValue(x)
	case uintptr:
		return uintAnyValue(uint64(x))
	case float32:
		return &v1.AnyValue{Value: &v1.AnyValue_DoubleValue{DoubleValue: float64(x)}}
	case float64:
		return &v1.AnyValue{Value: &v1.AnyValue_DoubleValue{DoubleValue: x}}
	default:
		return &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: fmt.Sprintf("%v", v)}}
	}
}

func uintAnyValue(v uint64) *v1.AnyValue {
	if v > math.MaxInt64 {
		return &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: fmt.Sprintf("%d", v)}}
	}
	return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: int64(v)}}
}

func keyValue(key string, value any) *v1.KeyValue {
	return &v1.KeyValue{Key: key, Value: ToAnyValue(value)}
}

func resourceAttributes(slug, uid string) []*v1.KeyValue {
	attrs := []*v1.KeyValue{}
	if slug != "" {
		attrs = append(attrs,
			keyValue(AttrHolonsSlug, slug),
			keyValue(AttrServiceName, slug),
		)
	}
	if uid != "" {
		attrs = append(attrs,
			keyValue(AttrHolonsInstanceUID, uid),
			keyValue(AttrServiceInstanceID, uid),
		)
	}
	return attrs
}

func sortedMapAttributes(m map[string]string) []*v1.KeyValue {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	attrs := make([]*v1.KeyValue, 0, len(keys))
	for _, k := range keys {
		attrs = append(attrs, keyValue(k, m[k]))
	}
	return attrs
}

func cloneAttributes(attrs []*v1.KeyValue) []*v1.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]*v1.KeyValue, len(attrs))
	for i, attr := range attrs {
		if attr == nil {
			continue
		}
		out[i] = proto.Clone(attr).(*v1.KeyValue)
	}
	return out
}

// ToProtoMetrics walks a RegistrySnapshot and emits OTLP-shaped metrics.
func ToProtoMetrics(snap RegistrySnapshot, slug, uid string, start time.Time) []*v1.Metric {
	out := make([]*v1.Metric, 0, len(snap.Counters)+len(snap.Gauges)+len(snap.Histograms))
	startNano := uint64(start.UnixNano())
	timeNano := uint64(snap.CapturedAt.UnixNano())
	for _, c := range snap.Counters {
		attrs := append(resourceAttributes(slug, uid), sortedMapAttributes(c.Labels)...)
		out = append(out, &v1.Metric{
			Name:        c.Name,
			Description: c.Help,
			Data: &v1.Metric_Sum{Sum: &v1.Sum{
				AggregationTemporality: v1.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				IsMonotonic:            true,
				DataPoints: []*v1.NumberDataPoint{{
					StartTimeUnixNano: startNano,
					TimeUnixNano:      timeNano,
					Attributes:        attrs,
					Value:             &v1.NumberDataPoint_AsInt{AsInt: c.Value},
				}},
			}},
		})
	}
	for _, g := range snap.Gauges {
		attrs := append(resourceAttributes(slug, uid), sortedMapAttributes(g.Labels)...)
		out = append(out, &v1.Metric{
			Name:        g.Name,
			Description: g.Help,
			Data: &v1.Metric_Gauge{Gauge: &v1.Gauge{
				DataPoints: []*v1.NumberDataPoint{{
					StartTimeUnixNano: startNano,
					TimeUnixNano:      timeNano,
					Attributes:        attrs,
					Value:             &v1.NumberDataPoint_AsDouble{AsDouble: g.Value},
				}},
			}},
		})
	}
	for _, h := range snap.Histograms {
		attrs := append(resourceAttributes(slug, uid), sortedMapAttributes(h.Labels)...)
		out = append(out, &v1.Metric{
			Name:        h.Name,
			Description: h.Help,
			Data: &v1.Metric_Histogram{Histogram: &v1.Histogram{
				AggregationTemporality: v1.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints: []*v1.HistogramDataPoint{{
					StartTimeUnixNano: startNano,
					TimeUnixNano:      timeNano,
					Attributes:        attrs,
					Count:             uint64(h.Snap.Total),
					Sum:               h.Snap.Sum,
					BucketCounts:      histogramBucketCounts(h.Snap),
					ExplicitBounds:    append([]float64(nil), h.Snap.Bounds...),
					Min:               h.Snap.Min,
					Max:               h.Snap.Max,
				}},
			}},
		})
	}
	return out
}

func histogramBucketCounts(s HistogramSnapshot) []uint64 {
	counts := make([]uint64, len(s.Counts)+1)
	var prev int64
	for i, c := range s.Counts {
		delta := c - prev
		if delta < 0 {
			delta = 0
		}
		counts[i] = uint64(delta)
		prev = c
	}
	tail := s.Total - prev
	if tail < 0 {
		tail = 0
	}
	counts[len(counts)-1] = uint64(tail)
	return counts
}

func histogramInfPlus() float64 { return math.Inf(1) }
