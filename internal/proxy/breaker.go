package proxy

import (
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker is a simple failure-counting breaker. When failures reach the
// threshold the circuit opens; after resetTimeout it allows a single probe
// request (half-open) to decide whether to close again or re-open.
type CircuitBreaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	threshold    int
	resetTimeout time.Duration
	lastFailure  time.Time
	// probeInFlight guards the half-open state so exactly one probe request
	// is allowed through at a time.
	probeInFlight bool
}

func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        StateClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Allow reports whether a request may proceed. It also performs the
// Open -> HalfOpen transition and admits a single probe. All decisions happen
// under one lock so concurrent callers cannot both slip through as "the probe".
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.probeInFlight = true
			return true // admit exactly one probe
		}
		return false
	case StateHalfOpen:
		// Only the single in-flight probe is allowed; everything else waits.
		return false
	default:
		return false
	}
}

// RecordSuccess closes the circuit after a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.probeInFlight = false
	cb.state = StateClosed
}

// RecordFailure increments the failure count and opens the circuit once the
// threshold is reached (or immediately if a half-open probe fails).
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == StateHalfOpen {
		// Probe failed: re-open immediately.
		cb.probeInFlight = false
		cb.state = StateOpen
		return
	}
	if cb.failures >= cb.threshold {
		cb.state = StateOpen
	}
}

// State returns the current breaker state (for metrics / health reporting).
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
