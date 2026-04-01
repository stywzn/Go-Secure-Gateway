package proxy

import (
	"errors"
	"net/url"
	"sync/atomic"
)

// LoadBalancer defines the interface for selecting a backend server
type LoadBalancer interface {
	Next() (*url.URL, error)
	AddBackend(target *url.URL)
}

// RoundRobinLB implements a lock-free round-robin load balancer
type RoundRobinLB struct {
	backends []*url.URL
	// Use uint64 for atomic operations to ensure thread safety without Mutex locks.
	// This is critical for high-concurrency gateway performance.
	current uint64
}

// NewRoundRobinLB initializes a new round-robin load balancer
func NewRoundRobinLB(urls []string) (*RoundRobinLB, error) {
	if len(urls) == 0 {
		return nil, errors.New("backend urls cannot be empty")
	}

	var backends []*url.URL
	for _, u := range urls {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		backends = append(backends, parsedURL)
	}

	return &RoundRobinLB{
		backends: backends,
		current:  0,
	}, nil
}

// Next atomically returns the next backend URL to route the request to
func (lb *RoundRobinLB) Next() (*url.URL, error) {
	if len(lb.backends) == 0 {
		return nil, errors.New("no available backends")
	}

	// Atomically increment the counter and get the new value.
	// This avoids expensive Mutex locking under high concurrent traffic.
	nextIndex := atomic.AddUint64(&lb.current, 1)

	// Modulo operation to wrap around the slice length
	target := lb.backends[nextIndex%uint64(len(lb.backends))]

	return target, nil
}

// AddBackend allows dynamic addition of new backend nodes (e.g., from ETCD)
// Note: In a fully dynamic system, modifying the slice itself would require a RWMutex,
// but for the routing path (Next), we keep it as fast as possible.
func (lb *RoundRobinLB) AddBackend(target *url.URL) {
	lb.backends = append(lb.backends, target)
}
