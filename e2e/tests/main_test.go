package tests

import (
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"gateway-e2e/internal/client"
	"gateway-e2e/internal/config"
	"gateway-e2e/internal/helpers"
)

// cfg holds the run configuration, loaded once for the whole suite.
var cfg = config.Load()

// TestMain waits until the gateway is ready before any test runs, so a
// still-starting stack produces a clear message instead of confusing failures.
func TestMain(m *testing.M) {
	if err := waitReady(cfg.BaseURL, 30*time.Second); err != nil {
		fmt.Printf("gateway not ready at %s: %v\n", cfg.BaseURL, err)
		fmt.Println("hint: start the stack first ->  docker compose up -d")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func waitReady(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	httpc := &http.Client{Timeout: 2 * time.Second}
	var last error
	for time.Now().Before(deadline) {
		resp, err := httpc.Get(baseURL + "/readyz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			last = err
		}
		time.Sleep(time.Second)
	}
	return last
}

// --- shared fixtures used across test files ---

// newClient returns a fresh, unauthenticated client.
func newClient() *client.Client {
	return client.New(cfg.BaseURL, cfg.Timeout)
}

// authClient returns a client carrying a valid token for the given user id.
func authClient(userID any) *client.Client {
	return newClient().WithToken(helpers.ValidToken(cfg.JWTSecret, userID))
}

var ipCounter uint32

// uniqueIP hands each caller a distinct source IP so their rate-limit buckets
// are isolated (see client.WithSourceIP).
func uniqueIP() string {
	n := atomic.AddUint32(&ipCounter, 1)
	return fmt.Sprintf("10.20.%d.%d", (n>>8)&0xff, (n&0xff)|1)
}
