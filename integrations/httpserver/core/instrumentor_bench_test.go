package httpservercore

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func benchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

// BenchmarkHandler_Baseline measures a plain, undecorated handler - the
// "without Argos" comparison point required alongside every instrumented
// benchmark.
func BenchmarkHandler_Baseline(b *testing.B) {
	handler := benchHandler()
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// BenchmarkHandler_Instrumented measures the same handler wrapped by
// Middleware, documenting the allocation cost Argos adds per request. The
// bulk of it is inherent to a real span+metric pipeline (context
// propagation, the attribute.Set the metrics API builds internally for
// WithAttributes, and the request's shallow copy for WithContext) rather
// than anything Argos-specific - there is no lower-allocation way to attach
// a span/metric to a context with the public OTel Go API as it stands.
func BenchmarkHandler_Instrumented(b *testing.B) {
	instr := New()
	handler := instr.Middleware(staticRoute("/users/{id}"))(benchHandler())
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// BenchmarkHandler_CaptureBodyOnError_Success documents that enabling
// WithCaptureBodyOnError adds no meaningful cost on the success path (the
// vast majority of requests) - the response body wrapper only buffers on
// Write, never reads ahead, and no capture attribute is ever attached here
// since the handler returns 200.
func BenchmarkHandler_CaptureBodyOnError_Success(b *testing.B) {
	instr := New(WithCaptureBodyOnError(1024))
	handler := instr.Middleware(staticRoute("/users/{id}"))(benchHandler())
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
