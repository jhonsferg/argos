package argosecho

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return exp
}

func TestMiddleware_ResolvesPathAndStatus(t *testing.T) {
	exp := setTracer(t)

	e := echo.New()
	e.Use(Middleware())
	e.GET("/items/:id", func(c echo.Context) error {
		return c.NoContent(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if want := "GET /items/:id"; spans[0].Name != want {
		t.Errorf("span name = %q, want %q", spans[0].Name, want)
	}
}

func TestMiddleware_ResolvesStatusFromReturnedError(t *testing.T) {
	exp := setTracer(t)

	e := echo.New()
	e.Use(Middleware())
	e.GET("/missing", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "nope")
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	var gotStatus bool
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "http.response.status_code" && kv.Value.AsInt64() == http.StatusNotFound {
			gotStatus = true
		}
	}
	if !gotStatus {
		t.Error("expected http.response.status_code=404 to be recorded from the returned error")
	}
}

func TestMiddleware_RecoversPanicAndWrites500(t *testing.T) {
	exp := setTracer(t)

	e := echo.New()
	e.Use(Middleware())
	e.GET("/boom", func(c echo.Context) error {
		panic("kaboom")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

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

func TestMiddleware_GenericErrorMapsToInternalError(t *testing.T) {
	setTracer(t)

	e := echo.New()
	e.Use(Middleware())
	e.GET("/fail", func(c echo.Context) error {
		return errors.New("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
