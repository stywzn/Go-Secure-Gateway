// Package config loads test-run settings from the environment so the same
// suite can target local docker-compose, a staging cluster, etc. — by only
// changing env vars, never code.
package config

import (
	"os"
	"time"
)

type Config struct {
	// BaseURL of the gateway under test.
	BaseURL string
	// JWTSecret must match the gateway's signing secret so the suite can mint
	// its own tokens (valid / expired / wrong-alg) for auth tests.
	JWTSecret string
	// Timeout for a single HTTP request.
	Timeout time.Duration
}

func Load() Config {
	return Config{
		BaseURL:   getenv("GATEWAY_BASE_URL", "http://localhost:8080"),
		JWTSecret: getenv("JWT_SECRET", "compose-dev-secret"),
		Timeout:   20 * time.Second,
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
