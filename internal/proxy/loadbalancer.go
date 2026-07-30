package proxy

import (
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
)

// LoadBalancer defines the interface for selecting a backend server.
type LoadBalancer interface {
	Next() (*url.URL, error)
	AddBackend(target *url.URL)
}

// RoundRobinLB implements a round-robin load balancer with lock-free reads.
//
// The backend slice is stored in an atomic.Value and treated as immutable:
// the fast routing path (Next) reads it without locking, while mutations
// (AddBackend) take a mutex and publish a fresh copy (copy-on-write). This
// keeps Next allocation- and lock-free under high concurrency while remaining
// data-race free when backends change.
type RoundRobinLB struct {
	backends atomic.Value // holds []*url.URL
	current  uint64
	writeMu  sync.Mutex // serializes copy-on-write mutations
}

// NewRoundRobinLB initializes a new round-robin load balancer.
func NewRoundRobinLB(urls []string) (*RoundRobinLB, error) {
	if len(urls) == 0 {
		return nil, errors.New("backend urls cannot be empty")
	}

	backends := make([]*url.URL, 0, len(urls))
	for _, u := range urls {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return nil, errors.New("backend url must be absolute (scheme + host): " + u)
		}
		backends = append(backends, parsedURL)
	}

	lb := &RoundRobinLB{}
	lb.backends.Store(backends)
	return lb, nil
}

// Next atomically returns the next backend URL to route the request to.
func (lb *RoundRobinLB) Next() (*url.URL, error) {
	backends, _ := lb.backends.Load().([]*url.URL)
	if len(backends) == 0 {
		return nil, errors.New("no available backends")
	}

	// Subtract 1 so the first request maps to index 0 rather than 1.
	nextIndex := atomic.AddUint64(&lb.current, 1) - 1
	return backends[nextIndex%uint64(len(backends))], nil
}

// AddBackend dynamically adds a new backend node (e.g. from service discovery).
// It publishes a new slice via copy-on-write so concurrent Next calls always
// observe a consistent, immutable snapshot.
func (lb *RoundRobinLB) AddBackend(target *url.URL) {
	lb.writeMu.Lock()
	defer lb.writeMu.Unlock()

	old, _ := lb.backends.Load().([]*url.URL)
	updated := make([]*url.URL, len(old), len(old)+1)
	copy(updated, old)
	updated = append(updated, target)
	lb.backends.Store(updated)
}
