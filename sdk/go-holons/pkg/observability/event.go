package observability

import (
	"context"
	"sync"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
)

const (
	EventInstanceSpawned = "instance.spawned"
	EventInstanceReady   = "instance.ready"
	EventInstanceExited  = "instance.exited"
	EventInstanceCrashed = "instance.crashed"
	EventSessionStarted  = "session.started"
	EventSessionEnded    = "session.ended"
	EventHandlerPanic    = "handler.panic"
	EventConfigReloaded  = "config.reloaded"
)

// EventBus is a bounded buffer + fan-out for event LogRecords.
type EventBus struct {
	mu     sync.Mutex
	buf    []LogRecord
	next   int
	filled bool
	cap    int
	closed bool

	subs []chan LogRecord
}

// NewEventBus constructs an EventBus with the given buffer capacity.
// Capacity must be positive; zero or negative falls back to 256.
func NewEventBus(capacity int) *EventBus {
	if capacity <= 0 {
		capacity = 256
	}
	return &EventBus{
		buf: make([]LogRecord, capacity),
		cap: capacity,
	}
}

// Emit pushes a new event record. Evicts the oldest when full.
func (b *EventBus) Emit(e LogRecord) {
	if b == nil {
		return
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.buf[b.next] = e
	b.next = (b.next + 1) % b.cap
	if !b.filled && b.next == 0 {
		b.filled = true
	}

	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
	b.mu.Unlock()
}

// Drain returns a snapshot of all resident event records in chronological order.
func (b *EventBus) Drain() []LogRecord {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.snapshotLocked()
}

// DrainSince returns event records with timestamp >= cutoff, in order.
func (b *EventBus) DrainSince(cutoff time.Time) []LogRecord {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	all := b.snapshotLocked()
	b.mu.Unlock()
	out := make([]LogRecord, 0, len(all))
	for _, e := range all {
		if !e.timestamp().Before(cutoff) {
			out = append(out, e)
		}
	}
	return out
}

func (b *EventBus) snapshotLocked() []LogRecord {
	n := b.cap
	if !b.filled {
		n = b.next
	}
	out := make([]LogRecord, n)
	if !b.filled {
		copy(out, b.buf[:n])
		return out
	}
	copy(out, b.buf[b.next:])
	copy(out[b.cap-b.next:], b.buf[:b.next])
	return out
}

// Watch returns a buffered channel that receives live event records.
func (b *EventBus) Watch(bufSize int) (<-chan LogRecord, func()) {
	if b == nil {
		ch := make(chan LogRecord)
		close(ch)
		return ch, func() {}
	}
	if bufSize <= 0 {
		bufSize = 32
	}
	ch := make(chan LogRecord, bufSize)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	stop := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, c := range b.subs {
			if c == ch {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				close(ch)
				return
			}
		}
	}
	return ch, stop
}

func (b *EventBus) replayAndWatch(cutoff time.Time, bufSize int) ([]LogRecord, <-chan LogRecord, func()) {
	if b == nil {
		ch := make(chan LogRecord)
		close(ch)
		return nil, ch, func() {}
	}
	if bufSize <= 0 {
		bufSize = 32
	}
	ch := make(chan LogRecord, bufSize)
	b.mu.Lock()
	if b.closed {
		close(ch)
		b.mu.Unlock()
		return nil, ch, func() {}
	}
	replay := b.snapshotLocked()
	if !cutoff.IsZero() {
		out := replay[:0]
		for _, e := range replay {
			if !e.timestamp().Before(cutoff) {
				out = append(out, e)
			}
		}
		replay = out
	}
	cpy := make([]LogRecord, len(replay))
	copy(cpy, replay)
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	stop := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, c := range b.subs {
			if c == ch {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				close(ch)
				return
			}
		}
	}
	return cpy, ch, stop
}

// Close disables future emits and closes every subscriber channel.
func (b *EventBus) Close() {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.closed = true
	for _, ch := range b.subs {
		close(ch)
	}
	b.subs = nil
	b.mu.Unlock()
}

// Emit publishes an event LogRecord through the active event bus.
func (o *Observability) Emit(ctx context.Context, eventName string, payload map[string]string, opts ...any) {
	if o == nil || !o.families[FamilyEvents] {
		return
	}
	private := false
	for _, opt := range opts {
		if isPrivateMarker(opt) {
			private = true
		}
	}
	sessionID, _ := fromContext(ctx)
	attrs := resourceAttributes(o.cfg.Slug, o.cfg.InstanceUID)
	if sessionID != "" {
		attrs = append(attrs, keyValue(AttrHolonsSessionID, sessionID))
	}
	if len(payload) > 0 {
		pcopy := make(map[string]string, len(payload))
		for k, v := range payload {
			if _, ok := o.redact[k]; ok {
				pcopy[k] = "<redacted>"
			} else {
				pcopy[k] = v
			}
		}
		attrs = append(attrs, sortedMapAttributes(pcopy)...)
	}
	now := time.Now()
	o.bus.Emit(newLogRecord(&v1.LogRecord{
		TimeUnixNano:         uint64(now.UnixNano()),
		ObservedTimeUnixNano: uint64(now.UnixNano()),
		SeverityNumber:       v1.SeverityNumber_SEVERITY_NUMBER_INFO,
		SeverityText:         "INFO",
		Body:                 ToAnyValue(eventName),
		Attributes:           attrs,
		EventName:            eventName,
	}, private))
}

// EventBus returns the active bus, or nil when events are off.
func (o *Observability) EventBus() *EventBus {
	if o == nil || !o.families[FamilyEvents] {
		return nil
	}
	return o.bus
}

// LogRing returns the active log ring, or nil when logs are off.
func (o *Observability) LogRing() *LogRing {
	if o == nil || !o.families[FamilyLogs] {
		return nil
	}
	return o.ringLogs
}
