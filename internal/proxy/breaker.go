package proxy

import (
	"sync"
	"time"
)

// Define circuit breaker states
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu           sync.RWMutex
	state        State
	failures     int
	threshold    int           // Max failures before opening the circuit
	resetTimeout time.Duration // Time to wait before entering Half-Open state
	lastFailure  time.Time
}

func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        StateClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Allow checks if the request is allowed to pass through
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	state := cb.state
	last := cb.lastFailure
	cb.mu.RUnlock()

	if state == StateClosed {
		return true
	}

	if state == StateOpen {
		// Check if the reset timeout has elapsed
		if time.Since(last) > cb.resetTimeout {
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.mu.Unlock()
			return true // Allow one test request to pass
		}
		return false
	}

	// If Half-Open, we strictly reject new requests until the test request finishes
	return false
}

// RecordSuccess resets the circuit breaker upon a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = StateClosed
}

// RecordFailure increments the failure count and opens the circuit if threshold is reached
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = StateOpen
	}
}
