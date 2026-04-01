package proxy

import (
	"Go-Secure-Gateway/internal/metrics"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"
)

type ProxyEngine struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
	cb     *CircuitBreaker
}

func NewProxyEngine(targetURL string) (*ProxyEngine, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	cb := NewCircuitBreaker(5, 10*time.Second)

	proxy.Transport = &http.Transport{
		MaxIdleConns:          100,              // Max global idle connections
		MaxIdleConnsPerHost:   100,              // Max idle connections per backend host
		IdleConnTimeout:       90 * time.Second, // Timeout for keeping idle connections alive
		TLSHandshakeTimeout:   10 * time.Second, // Timeout for TLS handshake
		ExpectContinueTimeout: 1 * time.Second,
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy Error] %v", err)
		// TODO: Trigger Circuit Breaker metrics here
		http.Error(w, "Backend service unavailable", http.StatusBadGateway)
	}

	return &ProxyEngine{
		target: target,
		proxy:  proxy,
		cb:     cb,
	}, nil
}

func (p *ProxyEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.cb.Allow() {
		http.Error(w, "503 Service Unavailable (Circuit Open)", http.StatusServiceUnavailable)
		return
	}

	// 2. Wrap ResponseWriter to capture status code
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	start := time.Now()

	// 3. Forward request to backend
	p.proxy.ServeHTTP(sw, r)

	// 4. Record Metrics
	duration := time.Since(start).Seconds()
	metrics.RequestCount.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(sw.status)).Inc()
	metrics.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)

	// 5. Update Circuit Breaker based on response status
	// Consider 5xx errors as failures, others as successes
	if sw.status >= 500 {
		p.cb.RecordFailure()
	} else {
		p.cb.RecordSuccess()
	}
}

// 	proxy := httputil.NewSingleHostReverseProxy(targetURL)

// 	return func(c *gin.Context) {
// 		log.Printf("[Gin 路由转发] %s %s -> %s", c.Request.Method, c.Request.URL.Path, targetHost)
// 		proxy.ServeHTTP(c.Writer, c.Request)
// 	}
// }
