package argosfiber

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
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

func TestMiddleware_ResolvesRouteAndStatus(t *testing.T) {
	exp := setTracer(t)

	app := fiber.New()
	app.Use(Middleware())
	app.Get("/items/:id", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
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

	app := fiber.New()
	app.Use(Middleware())
	app.Get("/missing", func(c *fiber.Ctx) error {
		return fiber.NewError(http.StatusNotFound, "nope")
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
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

	app := fiber.New()
	app.Use(Middleware())
	app.Get("/boom", func(c *fiber.Ctx) error {
		panic("kaboom")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
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

	app := fiber.New()
	app.Use(Middleware())
	app.Get("/fail", func(c *fiber.Ctx) error {
		return errors.New("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
