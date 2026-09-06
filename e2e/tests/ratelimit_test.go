package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rate limiting. Covers test points §2.2 (429 after burst) and §2.4 (per-IP
// isolation). Compose config (configs/config.docker.yaml): rps=10, burst=20.

func TestRateLimit_BlocksAfterBurst(t *testing.T) {
	c := authClient(1).WithSourceIP(uniqueIP())

	got429 := false
	for i := 0; i < 300; i++ {
		resp, err := c.Get("/interaction/ping")
		require.NoError(t, err)
		if resp.Status == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	assert.True(t, got429, "sustained requests from one IP should eventually hit 429")
}

// §2.4 — a different source IP has its own bucket, unaffected by another IP
// being throttled.
func TestRateLimit_PerIPIsolation(t *testing.T) {
	busy := authClient(1).WithSourceIP(uniqueIP())
	for i := 0; i < 300; i++ {
		resp, err := busy.Get("/interaction/ping")
		require.NoError(t, err)
		if resp.Status == http.StatusTooManyRequests {
			break
		}
	}

	// A brand-new IP should still be served.
	fresh := authClient(1).WithSourceIP(uniqueIP())
	resp, err := fresh.Get("/interaction/ping")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Status, "a fresh IP must not be affected by another IP's limit")
}
