package observability

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// PromHandler returns an http.Handler that renders the active
// Observability registry in Prometheus text exposition format
// (version 0.0.4). The handler is nil-safe; when Observability is
// disabled or metrics are off, it returns a 503 body with a short
// message so scrapers can distinguish it from a missing endpoint.
func PromHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		obs := Current()
		if !obs.Enabled(FamilyMetrics) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "# metrics family disabled (OP_OBS)\n")
			return
		}
		if !obs.Enabled(FamilyProm) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "# prom family disabled (OP_OBS)\n")
			return
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		snap := obs.Registry().Snapshot()
		injected := map[string]string{
			"slug": obs.Slug(),
		}
		if uid := obs.InstanceUID(); uid != "" {
			injected["instance_uid"] = uid
		}
		writePromExposition(w, snap, injected)
	})
}

// PromServer is an HTTP server exposing /metrics. Call Start to bind,
// Addr to get the effective /metrics URL, and Close to tear it down.
type PromServer struct {
	addr string

	mu     sync.Mutex
	lis    net.Listener
	server *http.Server
}

// NewPromServer constructs (but does not start) a PromServer that will
// bind to addr when Start is called. Pass ":0" for an ephemeral port.
func NewPromServer(addr string) *PromServer {
	return &PromServer{addr: addr}
}

// Start binds the configured address and begins serving in a background
// goroutine. Returns the effective metrics URL ("http://host:port/metrics")
// or an error. Idempotent in the sense that calling Start twice returns
// the previously bound address.
func (p *PromServer) Start() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lis != nil {
		return "http://" + p.lis.Addr().String() + "/metrics", nil
	}
	lis, err := net.Listen("tcp", p.addr)
	if err != nil {
		return "", fmt.Errorf("listen %s: %w", p.addr, err)
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", PromHandler())
	p.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	p.lis = lis
	go func() {
		_ = p.server.Serve(lis)
	}()
	return "http://" + lis.Addr().String() + "/metrics", nil
}

// Addr returns the effective bound /metrics URL, or empty if not started.
func (p *PromServer) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lis == nil {
		return ""
	}
	return "http://" + p.lis.Addr().String() + "/metrics"
}

// Close tears down the HTTP server and releases the listener. Safe to
// call multiple times.
func (p *PromServer) Close(ctx context.Context) error {
	p.mu.Lock()
	srv := p.server
	lis := p.lis
	p.server = nil
	p.lis = nil
	p.mu.Unlock()
	if srv == nil {
		return nil
	}
	if err := srv.Shutdown(ctx); err != nil {
		_ = lis.Close()
		return err
	}
	return nil
}

// writePromExposition renders the snapshot in Prometheus text format.
// Every sample receives the injected labels ("slug", "instance_uid")
// in addition to its own declared labels.
func writePromExposition(w io.Writer, snap RegistrySnapshot, injected map[string]string) {
	type nameGroup struct {
		name     string
		help     string
		typ      string
		counters []CounterSample
		gauges   []GaugeSample
		hists    []HistogramItem
	}
	groups := map[string]*nameGroup{}
	ensure := func(name, help, typ string) *nameGroup {
		g, ok := groups[name]
		if !ok {
			g = &nameGroup{name: name, help: help, typ: typ}
			groups[name] = g
		}
		if g.help == "" {
			g.help = help
		}
		return g
	}
	for _, c := range snap.Counters {
		ensure(c.Name, c.Help, "counter").counters = append(groups[c.Name].counters, c)
	}
	for _, g := range snap.Gauges {
		ensure(g.Name, g.Help, "gauge").gauges = append(groups[g.Name].gauges, g)
	}
	for _, h := range snap.Histograms {
		ensure(h.Name, h.Help, "histogram").hists = append(groups[h.Name].hists, h)
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		g := groups[name]
		fmt.Fprintf(w, "# HELP %s %s\n", name, promEscapeHelp(g.help))
		fmt.Fprintf(w, "# TYPE %s %s\n", name, g.typ)
		for _, c := range g.counters {
			fmt.Fprintf(w, "%s%s %d\n", c.Name, promLabels(mergeLabels(c.Labels, injected)), c.Value)
		}
		for _, gg := range g.gauges {
			fmt.Fprintf(w, "%s%s %s\n", gg.Name, promLabels(mergeLabels(gg.Labels, injected)), formatFloat(gg.Value))
		}
		for _, h := range g.hists {
			labels := mergeLabels(h.Labels, injected)
			for i, b := range h.Snap.Bounds {
				labels["le"] = formatFloat(b)
				fmt.Fprintf(w, "%s_bucket%s %d\n", h.Name, promLabels(labels), h.Snap.Counts[i])
			}
			labels["le"] = "+Inf"
			fmt.Fprintf(w, "%s_bucket%s %d\n", h.Name, promLabels(labels), h.Snap.Total)
			delete(labels, "le")
			fmt.Fprintf(w, "%s_sum%s %s\n", h.Name, promLabels(labels), formatFloat(h.Snap.Sum))
			fmt.Fprintf(w, "%s_count%s %d\n", h.Name, promLabels(labels), h.Snap.Total)
		}
	}
}

func mergeLabels(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range extra {
		if v == "" {
			continue
		}
		out[k] = v
	}
	for k, v := range base {
		out[k] = v
	}
	return out
}

func promLabels(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(promEscapeValue(m[k]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func promEscapeValue(s string) string {
	if !strings.ContainsAny(s, "\\\n\"") {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func promEscapeHelp(s string) string {
	if !strings.ContainsAny(s, "\\\n") {
		return s
	}
	return strings.NewReplacer("\\", `\\`, "\n", `\n`).Replace(s)
}

func formatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "+Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}
	if math.IsNaN(f) {
		return "NaN"
	}
	return fmt.Sprintf("%g", f)
}
