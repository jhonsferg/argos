package argosgin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return exp
}

func TestMiddleware_ResolvesGinFullPath(t *testing.T) {
	exp := setTracer(t)

	r := gin.New()
	r.Use(Middleware())
	r.GET("/items/:id", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if want := "GET /items/:id"; spans[0].Name != want {
		t.Errorf("span name = %q, want %q", spans[0].Name, want)
	}
}

func TestMiddleware_RecoversPanicAndWrites500(t *testing.T) {
	exp := setTracer(t)

	r := gin.New()
	r.Use(Middleware())
	r.GET("/boom", func(c *gin.Context) {
		panic("kaboom")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}
