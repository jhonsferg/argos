package argosgorillamux

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func BenchmarkMiddleware(b *testing.B) {
	r := mux.NewRouter()
	r.Use(Middleware())
	r.HandleFunc("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}
