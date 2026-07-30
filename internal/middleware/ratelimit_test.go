package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func init() { gin.SetMode(gin.TestMode) }

func TestRateLimitMiddleware_BlocksAfterBurst(t *testing.T) {
	// 0 refill rate, burst of 2: exactly two requests should pass.
	limiter := NewIPRateLimiter(rate.Limit(0), 2)
	defer limiter.Stop()

	r := gin.New()
	r.Use(RateLimitMiddleware(limiter))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	codes := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5555"
		r.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}

	if codes[0] != 200 || codes[1] != 200 {
		t.Errorf("first two requests should pass, got %v", codes)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Errorf("third request should be 429, got %d", codes[2])
	}
}

func TestIPRateLimiter_EvictStale(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(1), 1)
	defer limiter.Stop()

	limiter.getLimiter("9.9.9.9")
	limiter.mu.Lock()
	// Force the entry to look old.
	limiter.ips["9.9.9.9"].lastSeen = time.Now().Add(-time.Hour)
	limiter.mu.Unlock()

	limiter.evictStale()

	limiter.mu.Lock()
	_, exists := limiter.ips["9.9.9.9"]
	limiter.mu.Unlock()
	if exists {
		t.Error("stale entry should have been evicted")
	}
}
