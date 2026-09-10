package argoshttpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkRoundTrip_Baseline measures an uninstrumented transport - the
// "without Argos" comparison point.
func BenchmarkRoundTrip_Baseline(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	b.Cleanup(srv.Close)
	client := srv.Client()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}

// BenchmarkRoundTrip_Instrumented measures the same call through a
// Wrap-instrumented transport, documenting Argos's added allocation cost.
func BenchmarkRoundTrip_Instrumented(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	b.Cleanup(srv.Close)
	client := NewClient(srv.Client())

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}

// BenchmarkRoundTrip_CaptureBodyOnError_Success documents that enabling
// WithCaptureBodyOnError adds no meaningful cost on the success path - the
// request has no body to drain (GET) and the response is never drained
// since status/err don't indicate failure.
func BenchmarkRoundTrip_CaptureBodyOnError_Success(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	b.Cleanup(srv.Close)
	client := NewClient(srv.Client(), WithCaptureBodyOnError(1024))

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}
