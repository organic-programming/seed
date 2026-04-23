package observability

import (
	"sync"
	"time"
)

// LogRing is a thread-safe bounded ring buffer of LogEntry. Once full,
// pushing a new entry evicts the oldest. A snapshot can be read with
// Drain or DrainSince; readers of live entries subscribe via Watch.
type LogRing struct {
	mu      sync.Mutex
	entries []LogEntry
	next    int
	filled  bool
	cap     int

	subsMu sync.Mutex
	subs   []chan LogEntry
}

// NewLogRing constructs an empty ring with the given capacity. Capacity
// must be positive; zero or negative falls back to 1024.
func NewLogRing(capacity int) *LogRing {
	if capacity <= 0 {
		capacity = 1024
	}
	return &LogRing{
		entries: make([]LogEntry, capacity),
		cap:     capacity,
	}
}

// Push appends an entry. Evicts the oldest when full.
func (r *LogRing) Push(e LogEntry) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.entries[r.next] = e
	r.next = (r.next + 1) % r.cap
	if !r.filled && r.next == 0 {
		r.filled = true
	}
	r.mu.Unlock()

	// Fan-out to live subscribers. Non-blocking: a slow subscriber
	// drops rather than stalling the emitter.
	r.subsMu.Lock()
	for _, ch := range r.subs {
		select {
		case ch <- e:
		default:
		}
	}
	r.subsMu.Unlock()
}

// Len returns the number of entries currently held (<= cap).
func (r *LogRing) Len() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.filled {
		return r.cap
	}
	return r.next
}

// Cap returns the ring's configured capacity.
func (r *LogRing) Cap() int {
	if r == nil {
		return 0
	}
	return r.cap
}

// Drain returns a snapshot copy of all resident entries in
// chronological order (oldest first).
func (r *LogRing) Drain() []LogEntry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

// DrainSince returns entries with timestamp >= cutoff, in order.
func (r *LogRing) DrainSince(cutoff time.Time) []LogEntry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	all := r.snapshotLocked()
	out := all[:0]
	for _, e := range all {
		if !e.Timestamp.Before(cutoff) {
			out = append(out, e)
		}
	}
	cpy := make([]LogEntry, len(out))
	copy(cpy, out)
	return cpy
}

func (r *LogRing) snapshotLocked() []LogEntry {
	n := r.cap
	if !r.filled {
		n = r.next
	}
	out := make([]LogEntry, n)
	if !r.filled {
		copy(out, r.entries[:n])
		return out
	}
	// filled: start at r.next (oldest) and wrap.
	copy(out, r.entries[r.next:])
	copy(out[r.cap-r.next:], r.entries[:r.next])
	return out
}

// Watch returns a channel that receives live entries as they are
// pushed, starting after the call. The channel is buffered; a slow
// consumer drops without blocking the producer. Close the returned
// stop func to release the subscription.
func (r *LogRing) Watch(bufSize int) (<-chan LogEntry, func()) {
	if r == nil {
		ch := make(chan LogEntry)
		close(ch)
		return ch, func() {}
	}
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan LogEntry, bufSize)
	r.subsMu.Lock()
	r.subs = append(r.subs, ch)
	r.subsMu.Unlock()
	stop := func() {
		r.subsMu.Lock()
		defer r.subsMu.Unlock()
		for i, c := range r.subs {
			if c == ch {
				r.subs = append(r.subs[:i], r.subs[i+1:]...)
				close(ch)
				return
			}
		}
	}
	return ch, stop
}
