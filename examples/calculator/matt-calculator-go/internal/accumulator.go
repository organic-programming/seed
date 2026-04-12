package internal

import (
	"fmt"
	"strconv"
	"sync"
)

// Accumulator holds a running float64 total, safe for concurrent gRPC calls.
// It is zero-initialized and lives for the lifetime of the process.
type Accumulator struct {
	mu  sync.Mutex
	val float64
}

// Set resets the accumulator to v and returns the new value.
func (a *Accumulator) Set(v float64) float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.val = v
	return a.val
}

// Add increments the accumulator by v and returns (prev, new).
func (a *Accumulator) Add(v float64) (prev, next float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	prev = a.val
	a.val += v
	return prev, a.val
}

// Subtract decrements the accumulator by v and returns (prev, new).
func (a *Accumulator) Subtract(v float64) (prev, next float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	prev = a.val
	a.val -= v
	return prev, a.val
}

// Multiply scales the accumulator by by and returns (prev, new).
func (a *Accumulator) Multiply(by float64) (prev, next float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	prev = a.val
	a.val *= by
	return prev, a.val
}

// Divide divides the accumulator by by and returns (prev, new).
// Returns an error before acquiring the lock when by == 0.
func (a *Accumulator) Divide(by float64) (prev, next float64, err error) {
	if by == 0 {
		return 0, 0, fmt.Errorf("division by zero")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	prev = a.val
	a.val /= by
	return prev, a.val, nil
}

// Snapshot returns the current value without mutating it.
func (a *Accumulator) Snapshot() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.val
}

// FormatFloat formats a float64 without trailing zeros.
func FormatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
