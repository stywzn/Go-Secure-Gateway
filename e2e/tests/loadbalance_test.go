package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Load balancing. Covers test point §4.1: repeated calls to the 2-replica
// /storage route round-robin between backends, observable via X-Served-By.
func TestLoadBalance_RoundRobinAcrossReplicas(t *testing.T) {
	// Own source IP so the burst of requests doesn't disturb other tests'
	// rate-limit buckets.
	c := authClient(1).WithSourceIP(uniqueIP())

	seen := map[string]int{}
	const n = 12
	for i := 0; i < n; i++ {
		resp, err := c.Get("/storage/ping")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Status)
		seen[resp.Headers.Get("X-Served-By")]++
	}

	assert.Contains(t, seen, "storage-a", "storage-a should serve some requests")
	assert.Contains(t, seen, "storage-b", "storage-b should serve some requests")
}
