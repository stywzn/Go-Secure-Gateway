package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// ipEntry pairs a limiter with the last time its IP was seen, so idle entries
// can be evicted.
type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter keeps a token-bucket limiter per client IP. Stale entries are
// evicted by a background janitor so the map cannot grow without bound (which
// would otherwise be a memory-exhaustion vector under many distinct / spoofed
// source IPs).
type IPRateLimiter struct {
	ips map[string]*ipEntry
	mu  sync.Mutex
	r   rate.Limit
	b   int

	ttl  time.Duration
	stop chan struct{}
}

// NewIPRateLimiter creates a limiter allowing r requests/sec with burst b.
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	i := &IPRateLimiter{
		ips:  make(map[string]*ipEntry),
		r:    r,
		b:    b,
		ttl:  10 * time.Minute,
		stop: make(chan struct{}),
	}
	go i.cleanupLoop()
	return i
}

// getLimiter returns the limiter for ip, creating it on first use and
// refreshing its lastSeen timestamp.
func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	entry, exists := i.ips[ip]
	if !exists {
		entry = &ipEntry{limiter: rate.NewLimiter(i.r, i.b)}
		i.ips[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

func (i *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(i.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			i.evictStale()
		case <-i.stop:
			return
		}
	}
}

func (i *IPRateLimiter) evictStale() {
	cutoff := time.Now().Add(-i.ttl)
	i.mu.Lock()
	defer i.mu.Unlock()
	for ip, entry := range i.ips {
		if entry.lastSeen.Before(cutoff) {
			delete(i.ips, ip)
		}
	}
}

// Stop terminates the background janitor. Safe to call once.
func (i *IPRateLimiter) Stop() {
	close(i.stop)
}

// RateLimitMiddleware rejects requests from an IP that has exhausted its bucket.
func RateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !limiter.getLimiter(ip).Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "429 Too Many Requests",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
