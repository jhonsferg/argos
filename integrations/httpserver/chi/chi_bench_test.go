package argoschi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chirouter "github.com/go-chi/chi/v5"
)

func BenchmarkMiddleware(b *testing.B) {
	r := chirouter.NewRouter()
	r.Use(Middleware())
	r.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}
