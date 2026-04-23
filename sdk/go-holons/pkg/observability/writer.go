package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultRotateSize is the per-file byte threshold for disk rotation.
// Matches INSTANCES.md §Log Contract (16 MB).
const DefaultRotateSize = 16 * 1024 * 1024

// DefaultRotateKeep is the number of rotated files retained before the
// oldest is evicted. Matches INSTANCES.md §Log Contract (ring of 4).
const DefaultRotateKeep = 4

// DiskWriter writes JSONL records to a file with size-based rotation.
// Safe for concurrent use; serialises writes through an internal mutex.
type DiskWriter struct {
	mu       sync.Mutex
	path     string
	maxSize  int64
	keep     int
	file     *os.File
	written  int64
	rotated  int64 // cumulative bytes evicted via rotation
	openedAt time.Time
}

// NewDiskWriter prepares a writer for the given JSONL path with the
// spec's default rotation policy. The parent directory must exist
// (P3 op-side is responsible for creating it before the SDK starts).
func NewDiskWriter(path string) *DiskWriter {
	return &DiskWriter{path: path, maxSize: DefaultRotateSize, keep: DefaultRotateKeep}
}

// Open opens (or creates) the underlying file. Idempotent.
func (w *DiskWriter) Open() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(w.path), err)
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", w.path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.written = info.Size()
	w.openedAt = time.Now()
	return nil
}

// Close flushes and closes the file.
func (w *DiskWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// WriteJSON marshals v, appends a newline, and writes to disk. Returns
// the number of bytes written (including the newline). Rotates when
// the file would exceed maxSize.
func (w *DiskWriter) WriteJSON(v any) (int, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	return w.WriteLine(b)
}

// WriteLine appends payload followed by a newline. Payload must not
// contain embedded newlines (caller's responsibility).
func (w *DiskWriter) WriteLine(payload []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.openLocked(); err != nil {
			return 0, err
		}
	}
	needed := int64(len(payload)) + 1
	if w.written+needed > w.maxSize {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	n1, err := w.file.Write(payload)
	if err != nil {
		return n1, err
	}
	n2, err := w.file.Write([]byte{'\n'})
	n := n1 + n2
	w.written += int64(n)
	return n, err
}

func (w *DiskWriter) openLocked() error {
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(w.path), err)
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", w.path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.written = info.Size()
	return nil
}

func (w *DiskWriter) rotateLocked() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	// Shift .1→.2, .2→.3, ... up to .keep; drop .keep+1.
	for i := w.keep; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i)
		dst := fmt.Sprintf("%s.%d", w.path, i+1)
		if i == w.keep {
			// Remove the oldest rotate-out.
			if info, err := os.Stat(src); err == nil {
				w.rotated += info.Size()
			}
			_ = os.Remove(src)
			continue
		}
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}
	// Move the live file to .1
	if _, err := os.Stat(w.path); err == nil {
		_ = os.Rename(w.path, w.path+".1")
	}
	return w.openLocked()
}

// RotatedBytes returns the cumulative byte count evicted via rotation.
func (w *DiskWriter) RotatedBytes() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotated
}

// Path returns the live file path.
func (w *DiskWriter) Path() string { return w.path }

// ==== integration with Observability ====

// EnableDiskWriters wires disk writers into the active Observability,
// writing logs to <runDir>/stdout.log and events to
// <runDir>/events.jsonl when the respective families are enabled.
// Safe to call from the SDK startup path; subsequent calls replace the
// writers and close the previous ones.
func EnableDiskWriters(runDir string) error {
	obs := Current()
	if obs == nil || runDir == "" {
		return nil
	}
	if obs.Enabled(FamilyLogs) {
		lw := NewDiskWriter(filepath.Join(runDir, "stdout.log"))
		if err := lw.Open(); err != nil {
			return err
		}
		ring := obs.LogRing()
		ch, _ := ring.Watch(256)
		go func() {
			for e := range ch {
				rec := logEntryDiskRecord(e)
				_, _ = lw.WriteJSON(rec)
			}
		}()
	}
	if obs.Enabled(FamilyEvents) {
		ew := NewDiskWriter(filepath.Join(runDir, "events.jsonl"))
		if err := ew.Open(); err != nil {
			return err
		}
		bus := obs.EventBus()
		ch, _ := bus.Watch(64)
		go func() {
			for e := range ch {
				rec := eventDiskRecord(e)
				_, _ = ew.WriteJSON(rec)
			}
		}()
	}
	return nil
}

// logEntryDiskRecord shapes an on-disk representation that matches
// the multilog JSONL kind used by the root writer — a single line
// carries "kind":"log" plus the common fields.
func logEntryDiskRecord(e LogEntry) map[string]any {
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

// eventDiskRecord produces the on-disk shape for an event.
func eventDiskRecord(e Event) map[string]any {
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

// MetaJSON describes a running instance on disk. Written by the op
// spawn path into <runDir>/meta.json; read by op ps / op instances.
type MetaJSON struct {
	Slug             string    `json:"slug"`
	UID              string    `json:"uid"`
	PID              int       `json:"pid"`
	StartedAt        time.Time `json:"started_at"`
	Mode             string    `json:"mode"`
	Transport        string    `json:"transport"`
	Address          string    `json:"address"`
	MetricsAddr      string    `json:"metrics_addr,omitempty"`
	LogPath          string    `json:"log_path,omitempty"`
	LogBytesRotated  int64     `json:"log_bytes_rotated,omitempty"`
	OrganismUID      string    `json:"organism_uid,omitempty"`
	OrganismSlug     string    `json:"organism_slug,omitempty"`
	Default          bool      `json:"default,omitempty"`
}

// WriteMetaJSON serialises m to <runDir>/meta.json atomically.
func WriteMetaJSON(runDir string, m MetaJSON) error {
	if runDir == "" {
		return fmt.Errorf("WriteMetaJSON: empty runDir")
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(runDir, "meta.json")
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadMetaJSON reads a meta.json file from <runDir>.
func ReadMetaJSON(runDir string) (MetaJSON, error) {
	var m MetaJSON
	b, err := os.ReadFile(filepath.Join(runDir, "meta.json"))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}
