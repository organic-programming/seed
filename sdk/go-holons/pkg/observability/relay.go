package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Relay opens Logs / Events streams on a direct child's
// HolonObservability service and forwards every entry back through
// this holon's own rings after appending the child's ChainHop. The
// parent's own Logs stream then automatically includes the relayed
// entries because the ring is the source of truth for Logs.Serve.
//
// Usage: construct a Relay per direct child, call Start with a dialed
// client connection, Stop to tear down.
type Relay struct {
	childSlug string
	childUID  string
	conn      *grpc.ClientConn

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRelay constructs a relay targeting a direct child identified by
// its slug and UID. The caller owns the connection lifecycle.
func NewRelay(childSlug, childUID string, conn *grpc.ClientConn) *Relay {
	return &Relay{childSlug: childSlug, childUID: childUID, conn: conn}
}

// Start launches the relay goroutines. Safe to call once per Relay.
// Returns an error only if the initial stream open fails.
func (r *Relay) Start(ctx context.Context) error {
	if r == nil || r.conn == nil {
		return fmt.Errorf("observability.Relay: nil")
	}
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	client := v1.NewHolonObservabilityClient(r.conn)

	logsStream, err := client.Logs(ctx, &v1.LogsRequest{Follow: true})
	if err != nil {
		cancel()
		return fmt.Errorf("observability.Relay: open Logs: %w", err)
	}
	eventsStream, err := client.Events(ctx, &v1.EventsRequest{Follow: true})
	if err != nil {
		cancel()
		return fmt.Errorf("observability.Relay: open Events: %w", err)
	}

	r.wg.Add(2)
	go r.pumpLogs(logsStream)
	go r.pumpEvents(eventsStream)
	return nil
}

func (r *Relay) pumpLogs(stream v1.HolonObservability_LogsClient) {
	defer r.wg.Done()
	for {
		proto, err := stream.Recv()
		if err == io.EOF || err != nil {
			return
		}
		obs := Current()
		if obs == nil || !obs.Enabled(FamilyLogs) || obs.LogRing() == nil {
			continue
		}
		entry := FromProtoLogEntry(proto)
		entry.Chain = AppendDirectChild(entry.Chain, r.childSlug, r.childUID)
		obs.LogRing().Push(entry)
	}
}

func (r *Relay) pumpEvents(stream v1.HolonObservability_EventsClient) {
	defer r.wg.Done()
	for {
		proto, err := stream.Recv()
		if err == io.EOF || err != nil {
			return
		}
		obs := Current()
		if obs == nil || !obs.Enabled(FamilyEvents) || obs.EventBus() == nil {
			continue
		}
		ev := FromProtoEvent(proto)
		ev.Chain = AppendDirectChild(ev.Chain, r.childSlug, r.childUID)
		obs.EventBus().Emit(ev)
	}
}

// Stop cancels the relay and waits for its goroutines to exit.
func (r *Relay) Stop() {
	if r == nil || r.cancel == nil {
		return
	}
	r.cancel()
	r.wg.Wait()
}

// ChildSlug / ChildUID expose the relay's target identity.
func (r *Relay) ChildSlug() string { return r.childSlug }
func (r *Relay) ChildUID() string  { return r.childUID }

// --- Multilog writer ---------------------------------------------------------

// MultilogWriter serializes every signal observed by the root (local
// emissions + relayed entries) to a single JSONL file at
// <run_root>/<organism_slug>/<organism_uid>/multilog.jsonl, per
// OBSERVABILITY.md §Multilog Contract. The chain is enriched with the
// root's stream source (i.e., the entry's slug) so the line stands
// alone when read by external tooling.
//
// Rotation policy: 16 MB × ring of 4 (same as stdout.log).
type MultilogWriter struct {
	writer *DiskWriter

	mu       sync.Mutex
	started  bool
	stopLogs []func()
	stopEvs  []func()
}

// NewMultilogWriter prepares a writer rooted at <organismRunDir>.
// Pass the absolute path of the organism directory (the one that
// contains `members/`). No I/O happens until Start is called.
func NewMultilogWriter(organismRunDir string) *MultilogWriter {
	return &MultilogWriter{
		writer: NewDiskWriter(filepath.Join(organismRunDir, "multilog.jsonl")),
	}
}

// Start subscribes the writer to the active Observability's LogRing
// and EventBus so every entry that lands locally (either emitted by
// the root or appended by a Relay) is mirrored into the multilog.
// Safe to call only from the organism root.
func (m *MultilogWriter) Start() error {
	if m == nil {
		return fmt.Errorf("observability.MultilogWriter: nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	if err := m.writer.Open(); err != nil {
		return err
	}
	obs := Current()
	if obs == nil {
		return fmt.Errorf("observability.MultilogWriter: no active observability")
	}
	if obs.Enabled(FamilyLogs) && obs.LogRing() != nil {
		ring := obs.LogRing()
		ch, stop := ring.Watch(256)
		m.stopLogs = append(m.stopLogs, stop)
		go func() {
			for e := range ch {
				rec := multilogLog(e)
				_, _ = m.writer.WriteJSON(rec)
			}
		}()
	}
	if obs.Enabled(FamilyEvents) && obs.EventBus() != nil {
		bus := obs.EventBus()
		ch, stop := bus.Watch(64)
		m.stopEvs = append(m.stopEvs, stop)
		go func() {
			for e := range ch {
				rec := multilogEvent(e)
				_, _ = m.writer.WriteJSON(rec)
			}
		}()
	}
	m.started = true
	return nil
}

// Stop unsubscribes and closes the disk file.
func (m *MultilogWriter) Stop() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.stopLogs {
		s()
	}
	for _, s := range m.stopEvs {
		s()
	}
	m.stopLogs = nil
	m.stopEvs = nil
	m.started = false
	return m.writer.Close()
}

// multilogLog renders an in-memory LogEntry as the JSONL record the
// spec requires. The chain is enriched with the ENTRY's originator
// slug as the stream-source identifier when the entry was relayed.
// Locally emitted entries keep an empty chain.
func multilogLog(e LogEntry) map[string]any {
	rec := map[string]any{
		"kind":         "log",
		"ts":           e.Timestamp.UTC().Format(time.RFC3339Nano),
		"level":        e.Level.String(),
		"slug":         e.Slug,
		"instance_uid": e.InstanceUID,
		"message":      e.Message,
	}
	if e.SessionID != "" {
		rec["session_id"] = e.SessionID
	}
	if e.RPCMethod != "" {
		rec["rpc_method"] = e.RPCMethod
	}
	if len(e.Fields) > 0 {
		rec["fields"] = e.Fields
	}
	if e.Caller != "" {
		rec["caller"] = e.Caller
	}
	if len(e.Chain) > 0 {
		hops := make([]map[string]string, len(e.Chain))
		for i, h := range e.Chain {
			hops[i] = map[string]string{"slug": h.Slug, "instance_uid": h.InstanceUID}
		}
		rec["chain"] = hops
	}
	return rec
}

func multilogEvent(e Event) map[string]any {
	rec := map[string]any{
		"kind":         "event",
		"ts":           e.Timestamp.UTC().Format(time.RFC3339Nano),
		"type":         e.Type.String(),
		"slug":         e.Slug,
		"instance_uid": e.InstanceUID,
	}
	if e.SessionID != "" {
		rec["session_id"] = e.SessionID
	}
	if len(e.Payload) > 0 {
		rec["payload"] = e.Payload
	}
	if len(e.Chain) > 0 {
		hops := make([]map[string]string, len(e.Chain))
		for i, h := range e.Chain {
			hops[i] = map[string]string{"slug": h.Slug, "instance_uid": h.InstanceUID}
		}
		rec["chain"] = hops
	}
	return rec
}

// ReadMultilog is a helper for tests / tools that consumes a
// multilog file and returns each record as a decoded map, one per
// line.
func ReadMultilog(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var out []map[string]any
	for {
		var rec map[string]any
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

// --- Observability wiring helpers -------------------------------------------

// StartOrganismMultilog is a convenience for the root holon: it
// computes the organism run directory from the active Config and
// spins up a MultilogWriter. No-op when the holon is not an organism
// root or when OP_RUN_DIR is not set.
func StartOrganismMultilog() (*MultilogWriter, error) {
	obs := Current()
	if obs == nil || !obs.IsOrganismRoot() {
		return nil, nil
	}
	runDir := obs.cfg.RunDir
	if runDir == "" {
		return nil, nil
	}
	// <run_root>/<organism_slug>/<organism_uid>/
	// runDir here is already <run_root>/<slug>/<uid>/ when op run
	// injected it; the organism's multilog lives at that same level
	// when slug == organism_slug and uid == organism_uid.
	mw := NewMultilogWriter(runDir)
	if err := mw.Start(); err != nil {
		return nil, err
	}
	return mw, nil
}

// idleDeadline is deferred — kept here so linters don't complain
// about unused imports during partial build states.
var _ = time.Second
var _ = durationpb.New
