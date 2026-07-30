package proxy

import "net/http"

// statusWriter wraps http.ResponseWriter to capture the HTTP status code while
// preserving optional interfaces (Flusher, Hijacker) via Unwrap, which
// http.ResponseController uses. Without Unwrap, wrapping would silently break
// streaming responses (e.g. Server-Sent Events) and connection hijacking.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying ResponseWriter so http.ResponseController can
// reach its Flush/Hijack/SetDeadline methods.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
