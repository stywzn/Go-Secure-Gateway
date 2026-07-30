package tests

import (
	"net/http"
	"testing"

	"gateway-e2e/internal/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file is the reference example for the whole suite. New module test
// files (ratelimit_test.go, lb_test.go, breaker_test.go, ...) should follow
// the same shape: table-driven cases + t.Run subtests + testify assertions.
//
// Covers test points §1 (auth) and §3.7/§3.8 (identity propagation & anti-spoof).

// §1.1 — a valid token is accepted and the request reaches the backend.
func TestAuth_ValidTokenPasses(t *testing.T) {
	resp, err := authClient(9527).Get("/interaction/ping")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Status)
}

// §1.2~1.9 — every malformed / invalid token must be rejected with 401.
func TestAuth_Rejections(t *testing.T) {
	cases := []struct {
		name   string
		header string // full Authorization header value
	}{
		{"missing header", ""},
		{"wrong scheme", "Basic dXNlcjpwYXNz"},
		{"garbage token", "Bearer not.a.real.jwt"},
		{"expired", "Bearer " + helpers.ExpiredToken(cfg.JWTSecret)},
		{"missing exp claim", "Bearer " + helpers.NoExpToken(cfg.JWTSecret)},
		{"wrong signing secret", "Bearer " + helpers.WrongSecretToken()},
		{"alg=none forgery", "Bearer " + helpers.NoneAlgToken()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if tc.header != "" {
				headers["Authorization"] = tc.header
			}
			resp, err := newClient().Do(http.MethodGet, "/interaction/ping", nil, headers)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, resp.Status,
				"expected 401 for %q, body=%s", tc.name, resp.Body)
		})
	}
}

// §3.7 & §3.8 — the gateway injects the authenticated user id downstream and
// strips any client-supplied X-User-Id (anti-spoofing).
func TestAuth_UserIDPropagatedAndSpoofStripped(t *testing.T) {
	resp, err := authClient(12345).Do(
		http.MethodGet, "/interaction/ping", nil,
		map[string]string{"X-User-Id": "spoofed"}, // must be ignored
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)

	// The echo backend reflects the X-User-Id it actually received.
	var body struct {
		UserID string `json:"user_id"`
	}
	require.NoError(t, resp.JSON(&body))
	assert.Equal(t, "12345", body.UserID, "gateway should overwrite the spoofed id")
}
