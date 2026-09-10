package httpservercore

import "net/http"

// statusRecorder wraps a ResponseWriter to capture the status code actually
// written, defaulting to 200 per net/http's own WriteHeader contract (a
// handler that never calls WriteHeader gets an implicit 200 on first Write).
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

// Flush passes through to the underlying ResponseWriter's http.Flusher, if
// it implements one - needed for streaming/SSE handlers to keep working
// once wrapped.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the underlying ResponseWriter so http.ResponseController
// (and other type-assertion-based feature detection, e.g. http.Hijacker)
// can still reach it through this wrapper.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
