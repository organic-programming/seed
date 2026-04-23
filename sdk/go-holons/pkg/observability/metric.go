package observability

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// MetricKind identifies the type of a metric sample.
type MetricKind int

const (
	KindUnspecified MetricKind = iota
	KindCounter
	KindGauge
	KindHistogram
)

// Counter is a monotonically non-decreasing int64.
type Counter struct {
	name   string
	help   string
	labels map[string]string
	val    atomic.Int64
}

// Add increments the counter by n. Negative values are silently clamped
// to zero — counters do not decrease.
func (c *Counter) Add(n int64) {
	if c == nil {
		return
	}
	if n < 0 {
		return
	}
	c.val.Add(n)
}

// Inc increments the counter by 1. Equivalent to Add(1).
func (c *Counter) Inc() {
	if c == nil {
		return
	}
	c.val.Add(1)
}

// Value returns the current counter value.
func (c *Counter) Value() int64 {
	if c == nil {
		return 0
	}
	return c.val.Load()
}

// Name returns the metric's fully qualified name.
func (c *Counter) Name() string {
	if c == nil {
		return ""
	}
	return c.name
}

// Gauge is a point-in-time float64. Set and Add are both supported.
type Gauge struct {
	name   string
	help   string
	labels map[string]string

	mu  sync.Mutex
	val float64
}

// Set replaces the gauge's value.
func (g *Gauge) Set(v float64) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.val = v
	g.mu.Unlock()
}

// Add adds delta to the gauge's current value.
func (g *Gauge) Add(delta float64) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.val += delta
	g.mu.Unlock()
}

// Value returns the gauge's current value.
func (g *Gauge) Value() float64 {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	v := g.val
	g.mu.Unlock()
	return v
}

// Name returns the metric's fully qualified name.
func (g *Gauge) Name() string {
	if g == nil {
		return ""
	}
	return g.name
}

// Histogram is a cumulative bucket histogram with Prometheus semantics:
// each bucket counts observations whose value is <= bucket's upper
// bound. The implicit +Inf bucket is the total count.
//
// v1 uses fixed-boundary buckets (configurable). A later iteration can
// swap this for an HDR sketch without changing the public API.
type Histogram struct {
	name   string
	help   string
	labels map[string]string

	bounds []float64

	mu     sync.Mutex
	counts []int64 // parallel to bounds; counts[i] = # observations <= bounds[i]
	total  int64
	sum    float64
}

// DefaultBuckets covers the typical latency range for RPC work: 50µs
// to 60s on a roughly log-2 spacing.
var DefaultBuckets = []float64{
	50e-6, 100e-6, 250e-6, 500e-6,
	1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
	1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
}

// Observe records a new value. Thread-safe.
func (h *Histogram) Observe(v float64) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.total++
	h.sum += v
	for i, b := range h.bounds {
		if v <= b {
			h.counts[i]++
		}
	}
}

// ObserveDuration is a convenience for latency: observes v in seconds.
func (h *Histogram) ObserveDuration(d time.Duration) {
	h.Observe(d.Seconds())
}

// Snapshot returns a point-in-time view of the histogram's state.
// Safe to call from any goroutine.
func (h *Histogram) Snapshot() HistogramSnapshot {
	if h == nil {
		return HistogramSnapshot{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := HistogramSnapshot{
		Bounds: make([]float64, len(h.bounds)),
		Counts: make([]int64, len(h.counts)),
		Total:  h.total,
		Sum:    h.sum,
	}
	copy(out.Bounds, h.bounds)
	copy(out.Counts, h.counts)
	return out
}

// HistogramSnapshot is an immutable view of a histogram's state.
type HistogramSnapshot struct {
	Bounds []float64 // upper bounds, ascending
	Counts []int64   // cumulative counts aligned with Bounds
	Total  int64     // implicit +Inf bucket count
	Sum    float64
}

// Quantile returns an interpolated quantile estimate (q in [0,1]).
func (s HistogramSnapshot) Quantile(q float64) float64 {
	if s.Total == 0 {
		return math.NaN()
	}
	target := float64(s.Total) * q
	for i, c := range s.Counts {
		if float64(c) >= target {
			return s.Bounds[i]
		}
	}
	return math.Inf(1)
}

// Name returns the metric's fully qualified name.
func (h *Histogram) Name() string {
	if h == nil {
		return ""
	}
	return h.name
}

// sortedLabels returns the label key/value pairs sorted by key, in the
// format used by Prometheus exposition. Cached at registry level so
// this is called only on construction.
func sortedLabels(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		out = append(out, k, m[k])
	}
	return out
}
