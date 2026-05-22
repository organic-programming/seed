package observability

import (
	"sync"
	"time"
)

// LogRing is a thread-safe bounded ring buffer of LogRecord. Once full,
// pushing a new entry evicts the oldest. A snapshot can be read with
// Drain or DrainSince; readers of live entries subscribe via Watch.
type LogRing struct {
	mu      sync.Mutex
	entries []LogRecord
	next    int
	filled  bool
	cap     int

	subs []chan LogRecord
}

// NewLogRing constructs an empty ring with the given capacity. Capacity
// must be positive; zero or negative falls back to 1024.
func NewLogRing(capacity int) *LogRing {
	if capacity <= 0 {
		capacity = 1024
	}
	return &LogRing{
		entries: make([]LogRecord, capacity),
		cap:     capacity,
	}
}

// Push appends an entry. Evicts the oldest when full.
func (r *LogRing) Push(e LogRecord) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.entries[r.next] = e
	r.next = (r.next + 1) % r.cap
	if !r.filled && r.next == 0 {
		r.filled = true
	}
	// Fan-out to live subscribers. Non-blocking: a slow subscriber
	// drops rather than stalling the emitter.
	for _, ch := range r.subs {
		select {
		case ch <- e:
		default:
		}
	}
	r.mu.Unlock()
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
func (r *LogRing) Drain() []LogRecord {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

// DrainSince returns entries with timestamp >= cutoff, in order.
func (r *LogRing) DrainSince(cutoff time.Time) []LogRecord {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	all := r.snapshotLocked()
	out := all[:0]
	for _, e := range all {
		if !e.timestamp().Before(cutoff) {
			out = append(out, e)
		}
	}
	cpy := make([]LogRecord, len(out))
	copy(cpy, out)
	return cpy
}

func (r *LogRing) snapshotLocked() []LogRecord {
	n := r.cap
	if !r.filled {
		n = r.next
	}
	out := make([]LogRecord, n)
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
func (r *LogRing) Watch(bufSize int) (<-chan LogRecord, func()) {
	if r == nil {
		ch := make(chan LogRecord)
		close(ch)
		return ch, func() {}
	}
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan LogRecord, bufSize)
	r.mu.Lock()
	r.subs = append(r.subs, ch)
	r.mu.Unlock()
	stop := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
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

func (r *LogRing) replayAndWatch(cutoff time.Time, bufSize int) ([]LogRecord, <-chan LogRecord, func()) {
	if r == nil {
		ch := make(chan LogRecord)
		close(ch)
		return nil, ch, func() {}
	}
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan LogRecord, bufSize)
	r.mu.Lock()
	// Snapshot and subscription are under one lock: entries already in the
	// snapshot are not live-sent, and entries pushed after registration cannot
	// be missed while the caller drains the snapshot.
	replay := r.snapshotLocked()
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
	r.subs = append(r.subs, ch)
	r.mu.Unlock()
	stop := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for i, c := range r.subs {
			if c == ch {
				r.subs = append(r.subs[:i], r.subs[i+1:]...)
				close(ch)
				return
			}
		}
	}
	return cpy, ch, stop
}
