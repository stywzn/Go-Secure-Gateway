package proxy

import (
	"net/url"
	"sync"
	"testing"
)

func TestRoundRobinLB_Distribution(t *testing.T) {
	lb, err := NewRoundRobinLB([]string{
		"http://a:8080",
		"http://b:8080",
		"http://c:8080",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	counts := map[string]int{}
	for i := 0; i < 9; i++ {
		u, err := lb.Next()
		if err != nil {
			t.Fatalf("Next returned error: %v", err)
		}
		counts[u.Host]++
	}

	for _, host := range []string{"a:8080", "b:8080", "c:8080"} {
		if counts[host] != 3 {
			t.Errorf("host %s got %d requests, want 3", host, counts[host])
		}
	}
}

func TestRoundRobinLB_FirstRequestHitsFirstBackend(t *testing.T) {
	lb, _ := NewRoundRobinLB([]string{"http://a:8080", "http://b:8080"})
	u, _ := lb.Next()
	if u.Host != "a:8080" {
		t.Errorf("first request went to %s, want a:8080", u.Host)
	}
}

func TestNewRoundRobinLB_RejectsBadInput(t *testing.T) {
	if _, err := NewRoundRobinLB(nil); err == nil {
		t.Error("expected error for empty backend list")
	}
	if _, err := NewRoundRobinLB([]string{"not-a-url"}); err == nil {
		t.Error("expected error for non-absolute url")
	}
}

func TestRoundRobinLB_ConcurrentSafe(t *testing.T) {
	lb, _ := NewRoundRobinLB([]string{"http://a:8080", "http://b:8080"})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if _, err := lb.Next(); err != nil {
					t.Errorf("Next error: %v", err)
				}
			}
		}()
		go func(n int) {
			defer wg.Done()
			u, _ := url.Parse("http://dyn:9090")
			if n%25 == 0 {
				lb.AddBackend(u)
			}
		}(i)
	}
	wg.Wait()
}
