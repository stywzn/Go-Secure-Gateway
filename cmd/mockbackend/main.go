// Command mockbackend is a controllable test double used to exercise the
// gateway end-to-end. It is NOT a real service — its whole point is to be
// predictable and fully steerable from a test, so assertions never depend on
// a real backend's mood.
//
// Capabilities:
//   - Echo (default): reflects the request (path, X-User-Id, forwarded headers,
//     which replica served it) so you can assert prefix stripping / LB / auth
//     propagation.
//   - Control via query params on ANY path:
//     ?status=503   -> force that HTTP status (trigger 5xx / circuit breaker)
//     ?delay=2s     -> sleep before responding (trigger timeouts / latency)
//   - In-memory CRUD under /items for stateful end-to-end flow tests.
//   - POST /_reset clears the store so each test starts from a clean slate.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type server struct {
	name string

	mu     sync.Mutex
	nextID int
	items  map[int]map[string]any
}

func newServer(name string) *server {
	return &server{name: name, nextID: 1, items: map[int]map[string]any{}}
}

func main() {
	name := getenv("SERVICE_NAME", "mock")
	port := getenv("PORT", "8080")
	s := newServer(name)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("POST /_reset", s.reset)
	mux.HandleFunc("GET /items", s.listItems)
	mux.HandleFunc("POST /items", s.createItem)
	mux.HandleFunc("GET /items/{id}", s.getItem)
	mux.HandleFunc("PUT /items/{id}", s.putItem)
	mux.HandleFunc("DELETE /items/{id}", s.deleteItem)
	mux.HandleFunc("/", s.echo) // catch-all

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           s.withControl(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("mock backend %q listening on :%s", name, port)
	log.Fatal(srv.ListenAndServe())
}

// withControl applies ?delay and ?status before the real handler runs, so a
// test can force latency or any status code on any endpoint.
func (s *server) withControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if d := q.Get("delay"); d != "" {
			if dur, err := time.ParseDuration(d); err == nil {
				time.Sleep(dur)
			}
		}

		if code := q.Get("status"); code != "" {
			if n, err := strconv.Atoi(code); err == nil && n >= 100 && n < 600 {
				s.writeJSON(w, n, map[string]any{
					"forced_status": n,
					"service":       s.name,
					"path":          r.URL.Path,
				})
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": s.name})
}

func (s *server) echo(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"service":         s.name,
		"method":          r.Method,
		"path":            r.URL.Path,
		"query":           r.URL.RawQuery,
		"user_id":         r.Header.Get("X-User-Id"),
		"forwarded_for":   r.Header.Get("X-Forwarded-For"),
		"forwarded_proto": r.Header.Get("X-Forwarded-Proto"),
		"time":            time.Now().Format(time.RFC3339),
	})
	log.Printf("[%s] %s %s user=%q", s.name, r.Method, r.URL.Path, r.Header.Get("X-User-Id"))
}

// ---- In-memory CRUD ----

func (s *server) reset(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.items = map[int]map[string]any{}
	s.nextID = 1
	s.mu.Unlock()
	s.writeJSON(w, http.StatusOK, map[string]any{"reset": true, "service": s.name})
}

func (s *server) listItems(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]map[string]any, 0, len(s.items))
	for _, it := range s.items {
		list = append(list, it)
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": list, "count": len(list)})
}

func (s *server) createItem(w http.ResponseWriter, r *http.Request) {
	body, ok := s.decodeBody(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	body["id"] = id
	s.items[id] = body
	s.mu.Unlock()
	s.writeJSON(w, http.StatusCreated, body)
}

func (s *server) getItem(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	it, exists := s.items[id]
	s.mu.Unlock()
	if !exists {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found", "id": id})
		return
	}
	s.writeJSON(w, http.StatusOK, it)
}

func (s *server) putItem(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	body, ok := s.decodeBody(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	_, exists := s.items[id]
	if exists {
		body["id"] = id
		s.items[id] = body
	}
	s.mu.Unlock()
	if !exists {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found", "id": id})
		return
	}
	s.writeJSON(w, http.StatusOK, body)
}

func (s *server) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	_, exists := s.items[id]
	delete(s.items, id)
	s.mu.Unlock()
	if !exists {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found", "id": id})
		return
	}
	s.writeJSON(w, http.StatusNoContent, nil)
}

// ---- helpers ----

func (s *server) pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id must be an integer"})
		return 0, false
	}
	return id, true
}

func (s *server) decodeBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	body := map[string]any{}
	if r.Body == nil || r.ContentLength == 0 {
		return body, true
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return nil, false
	}
	return body, true
}

func (s *server) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Served-By", s.name)
	w.WriteHeader(code)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
