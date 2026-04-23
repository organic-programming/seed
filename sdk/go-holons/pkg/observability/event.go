package observability

import (
	"context"
	"sync"
	"time"
)

// EventType mirrors the proto EventType enum.
type EventType int32

const (
	EventTypeUnspecified    EventType = 0
	EventInstanceSpawned    EventType = 1
	EventInstanceReady      EventType = 2
	EventInstanceExited     EventType = 3
	EventInstanceCrashed    EventType = 4
	EventSessionStarted     EventType = 5
	EventSessionEnded       EventType = 6
	EventHandlerPanic       EventType = 7
	EventConfigReloaded     EventType = 8
)

// String returns the enum name.
func (t EventType) String() string {
	switch t {
	case EventInstanceSpawned:
		return "INSTANCE_SPAWNED"
	case EventInstanceReady:
		return "INSTANCE_READY"
	case EventInstanceExited:
		return "INSTANCE_EXITED"
	case EventInstanceCrashed:
		return "INSTANCE_CRASHED"
	case EventSessionStarted:
		return "SESSION_STARTED"
	case EventSessionEnded:
		return "SESSION_ENDED"
	case EventHandlerPanic:
		return "HANDLER_PANIC"
	case EventConfigReloaded:
		return "CONFIG_RELOADED"
	default:
		return "UNSPECIFIED"
	}
}

// Event is the in-memory representation of an EventInfo.
type Event struct {
	Timestamp   time.Time
	Type        EventType
	Slug        string
	InstanceUID string
	SessionID   string
	Payload     map[string]string
	Chain       []Hop
}

// EventBus is a bounded buffer + fan-out for events. Emit pushes into
// the buffer and broadcasts to live subscribers. Drain returns a
// snapshot (oldest first).
type EventBus struct {
	mu      sync.Mutex
	buf     []Event
	next    int
	filled  bool
	cap     int
	closed  bool

	subsMu sync.Mutex
	subs   []chan Event
}

// NewEventBus constructs an EventBus with the given buffer capacity.
// Capacity must be positive; zero or negative falls back to 256.
func NewEventBus(capacity int) *EventBus {
	if capacity <= 0 {
		capacity = 256
	}
	return &EventBus{
		buf: make([]Event, capacity),
		cap: capacity,
	}
}

// Emit pushes a new event. Evicts the oldest when full. Drops silently
// if the bus was closed.
func (b *EventBus) Emit(e Event) {
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
	b.mu.Unlock()

	b.subsMu.Lock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
	b.subsMu.Unlock()
}

// Drain returns a snapshot of all resident events in chronological order.
func (b *EventBus) Drain() []Event {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.snapshotLocked()
}

// DrainSince returns events with timestamp >= cutoff, in order.
func (b *EventBus) DrainSince(cutoff time.Time) []Event {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	all := b.snapshotLocked()
	b.mu.Unlock()
	out := make([]Event, 0, len(all))
	for _, e := range all {
		if !e.Timestamp.Before(cutoff) {
			out = append(out, e)
		}
	}
	return out
}

func (b *EventBus) snapshotLocked() []Event {
	n := b.cap
	if !b.filled {
		n = b.next
	}
	out := make([]Event, n)
	if !b.filled {
		copy(out, b.buf[:n])
		return out
	}
	copy(out, b.buf[b.next:])
	copy(out[b.cap-b.next:], b.buf[:b.next])
	return out
}

// Watch returns a channel that receives live events. The channel is
// buffered; a slow consumer drops. Call stop to release the subscription.
func (b *EventBus) Watch(bufSize int) (<-chan Event, func()) {
	if b == nil {
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}
	if bufSize <= 0 {
		bufSize = 32
	}
	ch := make(chan Event, bufSize)
	b.subsMu.Lock()
	b.subs = append(b.subs, ch)
	b.subsMu.Unlock()
	stop := func() {
		b.subsMu.Lock()
		defer b.subsMu.Unlock()
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

// Close disables future emits and closes every subscriber channel.
func (b *EventBus) Close() {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()

	b.subsMu.Lock()
	for _, ch := range b.subs {
		close(ch)
	}
	b.subs = nil
	b.subsMu.Unlock()
}

// Accessors on Observability.

// Emit is a convenience that fills in well-known fields from the
// Observability instance before publishing on the event bus. Payload
// fields listed in RedactedFields are replaced with "<redacted>".
func (o *Observability) Emit(ctx context.Context, typ EventType, payload map[string]string) {
	if o == nil || !o.families[FamilyEvents] {
		return
	}
	sessionID, _ := fromContext(ctx)
	// Apply redaction to payload.
	if len(o.redact) > 0 && len(payload) > 0 {
		pcopy := make(map[string]string, len(payload))
		for k, v := range payload {
			if _, ok := o.redact[k]; ok {
				pcopy[k] = "<redacted>"
			} else {
				pcopy[k] = v
			}
		}
		payload = pcopy
	}
	o.bus.Emit(Event{
		Timestamp:   time.Now(),
		Type:        typ,
		Slug:        o.cfg.Slug,
		InstanceUID: o.cfg.InstanceUID,
		SessionID:   sessionID,
		Payload:     payload,
	})
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
