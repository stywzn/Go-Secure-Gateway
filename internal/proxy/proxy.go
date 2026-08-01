package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"Go-Secure-Gateway/internal/config"
	"Go-Secure-Gateway/internal/metrics"
	"Go-Secure-Gateway/internal/middleware"
)

// ProxyEngine is a reverse proxy for a single route. It fronts one or more
// upstream backends (round-robin), guards them with a circuit breaker, and
// records request metrics.
type ProxyEngine struct {
	route           config.RouteConfig
	lb              *RoundRobinLB
	proxy           *httputil.ReverseProxy
	cb              *CircuitBreaker
	logger          *slog.Logger
	upstreamTimeout time.Duration
}

// NewProxyEngine builds a proxy engine for the given route. upstreamTimeout
// bounds how long the gateway waits for a backend response (0 disables it).
func NewProxyEngine(route config.RouteConfig, upstreamTimeout time.Duration, logger *slog.Logger) (*ProxyEngine, error) {
	if logger == nil {
		logger = slog.Default()
	}

	lb, err := NewRoundRobinLB(route.Backends())
	if err != nil {
		return nil, err
	}

	engine := &ProxyEngine{
		route:           route,
		lb:              lb,
		cb:              NewCircuitBreaker(5, 10*time.Second),
		logger:          logger,
		upstreamTimeout: upstreamTimeout,
	}

	rp := &httputil.ReverseProxy{
		Director: engine.director,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// A slow backend that exceeds upstreamTimeout surfaces as a
			// context deadline: return 504 Gateway Timeout. Everything else
			// (connection refused, reset, ...) is a 502 Bad Gateway.
			status := http.StatusBadGateway
			msg := "Backend service unavailable"
			if errors.Is(err, context.DeadlineExceeded) {
				status = http.StatusGatewayTimeout
				msg = "Upstream timed out"
			}
			engine.logger.Error("upstream request failed",
				"route", engine.route.PathPrefix,
				"path", r.URL.Path,
				"status", status,
				"err", err,
			)
			// The statusWriter records this 5xx as a failure, tripping the
			// circuit breaker after enough consecutive backend errors.
			http.Error(w, msg, status)
		},
	}
	engine.proxy = rp

	return engine, nil
}

// director rewrites the inbound request to point at the next chosen backend,
// optionally stripping the route prefix and propagating the authenticated
// user identity downstream.
func (p *ProxyEngine) director(req *http.Request) {
	target, err := p.lb.Next()
	if err != nil {
		// Validation guarantees at least one backend, so this is unexpected.
		p.logger.Error("no backend available", "route", p.route.PathPrefix)
		return
	}

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host

	path := req.URL.Path
	if p.route.StripPrefix {
		path = strings.TrimPrefix(path, p.route.PathPrefix)
		if path == "" || !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}
	req.URL.Path = singleJoiningSlash(target.Path, path)

	// Set standard forwarding headers.
	if req.Header.Get("X-Forwarded-Proto") == "" {
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		req.Header.Set("X-Forwarded-Proto", scheme)
	}
	if req.Header.Get("X-Forwarded-Host") == "" {
		req.Header.Set("X-Forwarded-Host", req.Host)
	}

	// Propagate the authenticated user id. Always delete any client-supplied
	// value first so callers cannot spoof their identity to the backend.
	req.Header.Del("X-User-Id")
	if uid := middleware.UserIDFromContext(req.Context()); uid != "" {
		req.Header.Set("X-User-Id", uid)
	}
}

func (p *ProxyEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.cb.Allow() {
		metrics.RequestCount.WithLabelValues(r.Method, p.route.PathPrefix, "503").Inc()
		http.Error(w, "503 Service Unavailable (Circuit Open)", http.StatusServiceUnavailable)
		return
	}

	// Bound how long we wait for the backend. When it is exceeded the
	// transport aborts the round-trip with context.DeadlineExceeded, which the
	// ErrorHandler turns into a 504.
	if p.upstreamTimeout > 0 {
		ctx, cancel := context.WithTimeout(r.Context(), p.upstreamTimeout)
		defer cancel()
		r = r.WithContext(ctx)
	}

	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	start := time.Now()

	p.proxy.ServeHTTP(sw, r)

	duration := time.Since(start).Seconds()
	// Use the stable route prefix (not the raw path) as the metric label to
	// avoid unbounded Prometheus cardinality.
	metrics.RequestCount.WithLabelValues(r.Method, p.route.PathPrefix, strconv.Itoa(sw.status)).Inc()
	metrics.RequestDuration.WithLabelValues(r.Method, p.route.PathPrefix).Observe(duration)

	if sw.status >= 500 {
		p.cb.RecordFailure()
	} else {
		p.cb.RecordSuccess()
	}
}

// singleJoiningSlash joins two URL path segments with exactly one slash
// between them (mirrors the helper in net/http/httputil).
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
