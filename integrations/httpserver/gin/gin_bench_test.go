package argosgin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func BenchmarkMiddleware(b *testing.B) {
	r := gin.New()
	r.Use(Middleware())
	r.GET("/items/:id", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}
