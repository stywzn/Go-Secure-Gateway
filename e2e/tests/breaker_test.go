package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Circuit breaker — the highest-value fault-injection test. Covers §5.
//
// Key detection trick: when the breaker is OPEN the gateway short-circuits with
// 503 *without* reaching the backend, so the response has no X-Served-By header.
// A backend-produced 5xx always carries X-Served-By. That header is how we tell
// "gateway tripped" apart from "backend errored".
//
// Gateway breaker config: 5 consecutive 5xx to open, 10s cooldown (proxy.go).

const breakerRoute = "/compute/run"
const cooldown = 11 * time.Second

func TestCircuitBreaker_OpensThenRecovers(t *testing.T) {
	if testing.Short() {
		t.Skip("breaker test waits ~11s for the cooldown window")
	}
	c := authClient(1).WithSourceIP(uniqueIP())

	// 1) Drive 5 consecutive backend 5xx to open the circuit.
	for i := 0; i < 5; i++ {
		resp, err := c.Get(breakerRoute + "?status=500")
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.Status)
		require.NotEmpty(t, resp.Headers.Get("X-Served-By"), "backend 5xx should reach the backend")
	}

	// 2) Now OPEN: the gateway short-circuits (503, no X-Served-By).
	resp, err := c.Get(breakerRoute)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.Status)
	assert.Empty(t, resp.Headers.Get("X-Served-By"), "open circuit must not reach the backend")

	// 3) After cooldown -> half-open admits one probe; make it succeed -> closed.
	time.Sleep(cooldown)
	resp, err = c.Get(breakerRoute)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Status)
	assert.NotEmpty(t, resp.Headers.Get("X-Served-By"), "recovered circuit should reach the backend again")
}
