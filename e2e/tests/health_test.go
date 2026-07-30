package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The simplest possible tests: public endpoints, no auth. Good template for
// understanding the client + assertions before moving to harder cases.
// Covers test points §6.1, §6.2, §6.4.

func TestHealth_Endpoints(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"liveness", "/healthz"},
		{"readiness", "/readyz"},
		{"metrics", "/metrics"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := newClient().Get(tc.path)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.Status)
		})
	}
}

func TestMetrics_ExposesGatewayCounter(t *testing.T) {
	resp, err := newClient().Get("/metrics")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)
	assert.Contains(t, string(resp.Body), "gateway_http_requests_total")
}
