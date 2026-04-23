package observability

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Registry owns all metrics registered in a single Observability
// instance. Metrics are keyed by name + sorted label values; repeated
// calls with the same key return the same metric.
type Registry struct {
	mu sync.RWMutex

	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   map[string]*Counter{},
		gauges:     map[string]*Gauge{},
		histograms: map[string]*Histogram{},
	}
}

// Counter returns or creates a counter with the given name and labels.
func (r *Registry) Counter(name, help string, labels map[string]string) *Counter {
	if r == nil {
		return nil
	}
	key := metricKey(name, labels)
	r.mu.RLock()
	if c, ok := r.counters[key]; ok {
		r.mu.RUnlock()
		return c
	}
	r.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[key]; ok {
		return c
	}
	c := &Counter{name: name, help: help, labels: copyLabels(labels)}
	r.counters[key] = c
	return c
}

// Gauge returns or creates a gauge with the given name and labels.
func (r *Registry) Gauge(name, help string, labels map[string]string) *Gauge {
	if r == nil {
		return nil
	}
	key := metricKey(name, labels)
	r.mu.RLock()
	if g, ok := r.gauges[key]; ok {
		r.mu.RUnlock()
		return g
	}
	r.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[key]; ok {
		return g
	}
	g := &Gauge{name: name, help: help, labels: copyLabels(labels)}
	r.gauges[key] = g
	return g
}

// Histogram returns or creates a histogram with the given name,
// labels, and bucket boundaries. If bounds is empty, DefaultBuckets
// is used.
func (r *Registry) Histogram(name, help string, labels map[string]string, bounds []float64) *Histogram {
	if r == nil {
		return nil
	}
	if len(bounds) == 0 {
		bounds = DefaultBuckets
	}
	key := metricKey(name, labels)
	r.mu.RLock()
	if h, ok := r.histograms[key]; ok {
		r.mu.RUnlock()
		return h
	}
	r.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[key]; ok {
		return h
	}
	// Copy bounds defensively; ensure ascending.
	bs := make([]float64, len(bounds))
	copy(bs, bounds)
	sort.Float64s(bs)
	h := &Histogram{
		name:   name,
		help:   help,
		labels: copyLabels(labels),
		bounds: bs,
		counts: make([]int64, len(bs)),
	}
	r.histograms[key] = h
	return h
}

// Snapshot returns a point-in-time view of all metrics in the registry.
type RegistrySnapshot struct {
	CapturedAt time.Time
	Counters   []CounterSample
	Gauges     []GaugeSample
	Histograms []HistogramItem
}

// CounterSample is a counter at a point in time.
type CounterSample struct {
	Name   string
	Help   string
	Labels map[string]string
	Value  int64
}

// GaugeSample is a gauge at a point in time.
type GaugeSample struct {
	Name   string
	Help   string
	Labels map[string]string
	Value  float64
}

// HistogramItem is a histogram at a point in time.
type HistogramItem struct {
	Name   string
	Help   string
	Labels map[string]string
	Snap   HistogramSnapshot
}

// Snapshot walks the registry and produces a stable, sorted view.
func (r *Registry) Snapshot() RegistrySnapshot {
	if r == nil {
		return RegistrySnapshot{CapturedAt: time.Now()}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap := RegistrySnapshot{CapturedAt: time.Now()}
	for _, c := range r.counters {
		snap.Counters = append(snap.Counters, CounterSample{
			Name:   c.name,
			Help:   c.help,
			Labels: copyLabels(c.labels),
			Value:  c.val.Load(),
		})
	}
	for _, g := range r.gauges {
		g.mu.Lock()
		v := g.val
		g.mu.Unlock()
		snap.Gauges = append(snap.Gauges, GaugeSample{
			Name:   g.name,
			Help:   g.help,
			Labels: copyLabels(g.labels),
			Value:  v,
		})
	}
	for _, h := range r.histograms {
		snap.Histograms = append(snap.Histograms, HistogramItem{
			Name:   h.name,
			Help:   h.help,
			Labels: copyLabels(h.labels),
			Snap:   h.Snapshot(),
		})
	}
	sort.Slice(snap.Counters, func(i, j int) bool { return snap.Counters[i].Name < snap.Counters[j].Name })
	sort.Slice(snap.Gauges, func(i, j int) bool { return snap.Gauges[i].Name < snap.Gauges[j].Name })
	sort.Slice(snap.Histograms, func(i, j int) bool { return snap.Histograms[i].Name < snap.Histograms[j].Name })
	return snap
}

func metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	for _, k := range keys {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}
	return b.String()
}

func copyLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		out[k] = v
	}
	return out
}

// Convenience accessors on Observability that funnel through Current
// when appropriate.

// Counter returns a counter from the active registry. Returns nil
// when metrics are disabled — callers can use the nil-safe
// (*Counter).Add method regardless.
func (o *Observability) Counter(name, help string, labels map[string]string) *Counter {
	if o == nil || !o.families[FamilyMetrics] {
		return nil
	}
	return o.registry.Counter(name, help, labels)
}

// Gauge returns a gauge from the active registry.
func (o *Observability) Gauge(name, help string, labels map[string]string) *Gauge {
	if o == nil || !o.families[FamilyMetrics] {
		return nil
	}
	return o.registry.Gauge(name, help, labels)
}

// Histogram returns a histogram from the active registry.
func (o *Observability) Histogram(name, help string, labels map[string]string, bounds []float64) *Histogram {
	if o == nil || !o.families[FamilyMetrics] {
		return nil
	}
	return o.registry.Histogram(name, help, labels, bounds)
}

// Registry returns the active registry, or nil when metrics are off.
func (o *Observability) Registry() *Registry {
	if o == nil || !o.families[FamilyMetrics] {
		return nil
	}
	return o.registry
}
