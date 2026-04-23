package observability

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestParseOPOBS_Basic(t *testing.T) {
	tests := []struct {
		in   string
		want map[Family]bool
	}{
		{"", map[Family]bool{}},
		{"logs", map[Family]bool{FamilyLogs: true}},
		{"logs,metrics", map[Family]bool{FamilyLogs: true, FamilyMetrics: true}},
		{"all", map[Family]bool{FamilyLogs: true, FamilyMetrics: true, FamilyEvents: true, FamilyProm: true}},
		{"all,otel", map[Family]bool{FamilyLogs: true, FamilyMetrics: true, FamilyEvents: true, FamilyProm: true}}, // otel silently dropped in v1
		{"unknown", map[Family]bool{}},
	}
	for _, tc := range tests {
		got := parseOPOBS(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("parseOPOBS(%q) len=%d, want %d; got=%v", tc.in, len(got), len(tc.want), got)
			continue
		}
		for k, v := range tc.want {
			if got[k] != v {
				t.Errorf("parseOPOBS(%q)[%v]=%v, want %v", tc.in, k, got[k], v)
			}
		}
	}
}

func TestCheckEnv_OtelRejected(t *testing.T) {
	t.Setenv("OP_OBS", "logs,otel")
	err := CheckEnv()
	if err == nil {
		t.Fatal("expected error for otel in v1")
	}
	if ite, ok := err.(*InvalidTokenError); !ok || ite.Token != "otel" {
		t.Fatalf("expected InvalidTokenError{Token:otel}, got %v", err)
	}
}

func TestCheckEnv_UnknownRejected(t *testing.T) {
	t.Setenv("OP_OBS", "bogus")
	err := CheckEnv()
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}

func TestCheckEnv_Valid(t *testing.T) {
	t.Setenv("OP_OBS", "logs,metrics,events,prom,all")
	if err := CheckEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigure_Disabled_ZeroCost(t *testing.T) {
	Reset()
	os.Unsetenv("OP_OBS")
	obs := Configure(Config{Slug: "test"})
	defer obs.Close()

	if obs.Enabled(FamilyLogs) || obs.Enabled(FamilyMetrics) || obs.Enabled(FamilyEvents) {
		t.Error("no family should be enabled with empty OP_OBS")
	}
	l := obs.Logger("test")
	l.Info("noop", "k", "v") // should be a no-op

	c := obs.Counter("test_total", "", nil)
	if c != nil {
		t.Error("counter should be nil when metrics disabled")
	}
	c.Add(1) // nil-safe
	c.Inc()
	if c.Value() != 0 {
		t.Error("nil counter must read 0")
	}
}

func TestConfigure_LogsFamily(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "logs")
	obs := Configure(Config{Slug: "g", InstanceUID: "uid"})
	defer obs.Close()

	if !obs.Enabled(FamilyLogs) {
		t.Fatal("logs family not enabled")
	}
	l := obs.Logger("recipe.runner")
	l.Info("recipe started", "name", "greeter")
	l.Warn("retry budget low", "remaining", 3)

	entries := obs.LogRing().Drain()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Message != "recipe started" || entries[1].Message != "retry budget low" {
		t.Errorf("unexpected messages: %+v", entries)
	}
	if entries[0].Slug != "g" || entries[0].InstanceUID != "uid" {
		t.Errorf("well-known fields not injected: %+v", entries[0])
	}
	if entries[0].Fields["name"] != "greeter" {
		t.Errorf("user field missing: %+v", entries[0].Fields)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "logs")
	obs := Configure(Config{Slug: "g", DefaultLogLevel: LevelWarn})
	defer obs.Close()

	l := obs.Logger("filter")
	l.Info("should be dropped")
	l.Warn("should pass")
	l.Error("should pass")

	if n := obs.LogRing().Len(); n != 2 {
		t.Fatalf("expected 2 entries at WARN+, got %d", n)
	}
}

func TestLogger_RedactFields(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "logs")
	obs := Configure(Config{Slug: "g", RedactedFields: []string{"password", "api_key"}})
	defer obs.Close()

	obs.Logger("login").Info("authenticated", "user", "bob", "password", "secret", "api_key", "abc123")

	e := obs.LogRing().Drain()[0]
	if e.Fields["user"] != "bob" {
		t.Errorf("user field unexpectedly altered: %v", e.Fields["user"])
	}
	if e.Fields["password"] != "<redacted>" || e.Fields["api_key"] != "<redacted>" {
		t.Errorf("redaction failed: %v", e.Fields)
	}
}

func TestRing_PushEvictsOldest(t *testing.T) {
	r := NewLogRing(3)
	for i := 0; i < 5; i++ {
		r.Push(LogEntry{Message: string(rune('a' + i))})
	}
	entries := r.Drain()
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}
	if entries[0].Message != "c" || entries[1].Message != "d" || entries[2].Message != "e" {
		t.Errorf("wrong order: %+v", entries)
	}
}

func TestRing_Watch(t *testing.T) {
	r := NewLogRing(10)
	ch, stop := r.Watch(4)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	received := make([]LogEntry, 0, 3)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			e := <-ch
			received = append(received, e)
		}
	}()

	r.Push(LogEntry{Message: "1"})
	r.Push(LogEntry{Message: "2"})
	r.Push(LogEntry{Message: "3"})

	wg.Wait()
	if len(received) != 3 {
		t.Fatalf("expected 3 live entries, got %d", len(received))
	}
}

func TestCounter_Atomic(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "metrics")
	obs := Configure(Config{Slug: "g"})
	defer obs.Close()

	c := obs.Counter("test_total", "", nil)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Inc()
			}
		}()
	}
	wg.Wait()
	if c.Value() != 10000 {
		t.Errorf("expected 10000, got %d", c.Value())
	}
}

func TestHistogram_Percentile(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "metrics")
	obs := Configure(Config{Slug: "g"})
	defer obs.Close()

	h := obs.Histogram("latency_seconds", "", nil, []float64{1e-3, 1e-2, 1e-1, 1.0})
	for i := 0; i < 900; i++ {
		h.Observe(0.5e-3) // below smallest bucket (0.001); counts in bucket[0]
	}
	for i := 0; i < 100; i++ {
		h.Observe(0.5) // counts in bucket[3] (1.0)
	}
	snap := h.Snapshot()
	p50 := snap.Quantile(0.50)
	p99 := snap.Quantile(0.99)
	if p50 != 1e-3 {
		t.Errorf("expected p50=1e-3 (first bucket), got %v", p50)
	}
	if p99 != 1.0 {
		t.Errorf("expected p99=1.0, got %v", p99)
	}
}

func TestEventBus_FanOut(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "events")
	obs := Configure(Config{Slug: "g", InstanceUID: "uid"})
	defer obs.Close()

	bus := obs.EventBus()
	ch1, stop1 := bus.Watch(8)
	defer stop1()
	ch2, stop2 := bus.Watch(8)
	defer stop2()

	obs.Emit(context.Background(), EventInstanceReady, map[string]string{"listener": "stdio://"})
	time.Sleep(5 * time.Millisecond) // let subscribers receive

	select {
	case e := <-ch1:
		if e.Type != EventInstanceReady {
			t.Errorf("ch1: unexpected event: %+v", e)
		}
	default:
		t.Error("ch1 did not receive event")
	}
	select {
	case e := <-ch2:
		if e.Type != EventInstanceReady {
			t.Errorf("ch2: unexpected event: %+v", e)
		}
	default:
		t.Error("ch2 did not receive event")
	}

	// Drain returns the full buffer history.
	ev := bus.Drain()
	if len(ev) != 1 {
		t.Errorf("expected 1 event in drain, got %d", len(ev))
	}
}

func TestChain_AppendAndEnrich(t *testing.T) {
	// Wire chain: start with [], direct-child relay appends.
	c1 := AppendDirectChild(nil, "gabriel-greeting-rust", "1c2d")
	if len(c1) != 1 || c1[0].Slug != "gabriel-greeting-rust" {
		t.Fatalf("append direct child failed: %+v", c1)
	}

	// Multilog enrichment appends stream source.
	c2 := EnrichForMultilog(c1, "gabriel-greeting-go", "ea34")
	if len(c2) != 2 || c2[0].Slug != "gabriel-greeting-rust" || c2[1].Slug != "gabriel-greeting-go" {
		t.Fatalf("multilog enrichment failed: %+v", c2)
	}
	// Original wire chain must be unchanged.
	if len(c1) != 1 {
		t.Fatalf("wire chain mutated: %+v", c1)
	}
}

func TestIsOrganismRoot(t *testing.T) {
	Reset()
	os.Unsetenv("OP_OBS")
	obs := Configure(Config{Slug: "g", InstanceUID: "x", OrganismUID: "x"})
	defer obs.Close()
	if !obs.IsOrganismRoot() {
		t.Error("expected root when OP_ORGANISM_UID == OP_INSTANCE_UID")
	}
	Reset()
	obs2 := Configure(Config{Slug: "g", InstanceUID: "x", OrganismUID: "y"})
	defer obs2.Close()
	if obs2.IsOrganismRoot() {
		t.Error("expected not-root when uids differ")
	}
}

func TestCurrent_SafeWhenUnset(t *testing.T) {
	Reset()
	c := Current()
	if c == nil {
		t.Fatal("Current should never return nil; disabled stub expected")
	}
	c.Logger("anything").Info("no-op call path")
	c.Emit(nil, EventInstanceReady, nil)
}

var _ = time.Second
