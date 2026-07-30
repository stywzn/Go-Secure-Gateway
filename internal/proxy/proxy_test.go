package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Go-Secure-Gateway/internal/config"
)

func TestProxyEngine_StripPrefixAndForward(t *testing.T) {
	var gotPath, gotUserID, gotProto string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUserID = r.Header.Get("X-User-Id")
		gotProto = r.Header.Get("X-Forwarded-Proto")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	engine, err := NewProxyEngine(config.RouteConfig{
		PathPrefix:  "/storage",
		TargetURL:   backend.URL,
		StripPrefix: true,
	}, nil)
	if err != nil {
		t.Fatalf("NewProxyEngine: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/storage/files/1", nil)
	// A malicious client tries to spoof its identity; it must be stripped
	// because no authenticated user is on the request context.
	req.Header.Set("X-User-Id", "spoofed")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotPath != "/files/1" {
		t.Errorf("upstream path = %q, want /files/1 (prefix should be stripped)", gotPath)
	}
	if gotUserID != "" {
		t.Errorf("spoofed X-User-Id should be stripped, got %q", gotUserID)
	}
	if gotProto != "http" {
		t.Errorf("X-Forwarded-Proto = %q, want http", gotProto)
	}
}

func TestProxyEngine_NoStripKeepsPath(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	defer backend.Close()

	engine, _ := NewProxyEngine(config.RouteConfig{
		PathPrefix: "/interaction",
		TargetURL:  backend.URL,
	}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/interaction/ping", nil)
	engine.ServeHTTP(w, req)

	if gotPath != "/interaction/ping" {
		t.Errorf("upstream path = %q, want /interaction/ping", gotPath)
	}
}

func TestProxyEngine_CircuitBreakerOpensOn5xx(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	engine, _ := NewProxyEngine(config.RouteConfig{
		PathPrefix: "/x",
		TargetURL:  backend.URL,
	}, nil)

	// Default breaker threshold is 5 consecutive failures.
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	}

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 once circuit is open, got %d", w.Code)
	}
}
