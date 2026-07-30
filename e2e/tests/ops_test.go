package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Operational endpoints. Covers §6.7/§6.8: the debug token minting endpoint is
// reachable only when the gateway runs with debug=true (as docker-compose does).
func TestOps_DebugTokenAvailableInDebugMode(t *testing.T) {
	resp, err := newClient().Get("/debug/token")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status,
		"compose runs with debug=true, so /debug/token should return a token")

	var body struct {
		Token string `json:"token"`
	}
	require.NoError(t, resp.JSON(&body))
	assert.NotEmpty(t, body.Token)
}
