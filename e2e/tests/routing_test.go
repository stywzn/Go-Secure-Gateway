package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Routing / reverse-proxy behavior. Covers test points §3.

type echoResp struct {
	Service string `json:"service"`
	Path    string `json:"path"`
	Query   string `json:"query"`
}

// §3.1 — /interaction is configured strip_prefix:false, so the backend sees
// the full path including the prefix.
func TestRouting_PrefixNotStripped(t *testing.T) {
	resp, err := authClient(1).Get("/interaction/ping")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)

	var body echoResp
	require.NoError(t, resp.JSON(&body))
	assert.Equal(t, "/interaction/ping", body.Path)
}

// §3.2 — /storage is strip_prefix:true, so the prefix is removed upstream.
func TestRouting_PrefixStripped(t *testing.T) {
	resp, err := authClient(1).Get("/storage/ping")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)

	var body echoResp
	require.NoError(t, resp.JSON(&body))
	assert.Equal(t, "/ping", body.Path, "prefix /storage should be stripped")
}

// §3.4 — query string is forwarded intact.
func TestRouting_QueryPassthrough(t *testing.T) {
	resp, err := authClient(1).Get("/compute/run?foo=bar&n=42")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)

	var body echoResp
	require.NoError(t, resp.JSON(&body))
	assert.Contains(t, body.Query, "foo=bar")
}

// §3.6 — an unconfigured prefix has no route and returns 404.
func TestRouting_UnknownRouteIs404(t *testing.T) {
	resp, err := authClient(1).Get("/does-not-exist/x")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.Status)
}
