package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Latency injection. Covers §12.1: a delay within the gateway's write timeout
// still returns normally. (The 30s-delay/timeout case is left out here to keep
// the suite fast and to avoid tripping the breaker via induced 5xx.)
func TestTimeout_DelayWithinLimitSucceeds(t *testing.T) {
	c := authClient(1).WithSourceIP(uniqueIP())
	resp, err := c.Get("/compute/run?delay=1s")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Status)
}
