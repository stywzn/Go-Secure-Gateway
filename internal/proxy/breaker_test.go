package proxy

import (
	"testing"
	"time"
)

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)

	if !cb.Allow() {
		t.Fatal("breaker should start closed")
	}
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.Allow() {
		t.Error("breaker should be open after reaching threshold")
	}
	if cb.State() != StateOpen {
		t.Errorf("state = %s, want open", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenAdmitsSingleProbe(t *testing.T) {
	cb := NewCircuitBreaker(1, 20*time.Millisecond)
	cb.RecordFailure() // opens immediately (threshold 1)

	if cb.Allow() {
		t.Fatal("breaker should be open right after failure")
	}

	time.Sleep(30 * time.Millisecond) // wait past resetTimeout

	// First call transitions to half-open and admits the probe.
	if !cb.Allow() {
		t.Fatal("expected probe to be admitted after reset timeout")
	}
	// A second concurrent-style call must be rejected while the probe is in flight.
	if cb.Allow() {
		t.Error("second request should be rejected during half-open probe")
	}
}

func TestCircuitBreaker_ProbeSuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)

	cb.Allow() // admit probe -> half-open
	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Errorf("state = %s, want closed after successful probe", cb.State())
	}
	if !cb.Allow() {
		t.Error("breaker should allow traffic after closing")
	}
}

func TestCircuitBreaker_ProbeFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)

	cb.Allow() // admit probe -> half-open
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("state = %s, want open after failed probe", cb.State())
	}
}
