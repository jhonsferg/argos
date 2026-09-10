package httpservercore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestStartWithCarrier_ExtractsFromArbitraryCarrier covers the generic path
// non-net/http frameworks (e.g. fasthttp-based ones) use directly, since
// Start itself only exercises it indirectly via propagation.HeaderCarrier.
func TestStartWithCarrier_ExtractsFromArbitraryCarrier(t *testing.T) {
	exp := setTracer(t)
	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(prevProp) })

	instr := New()
	parentTraceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	parentSpanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	carrier := propagation.MapCarrier{
		"traceparent": "00-" + parentTraceID.String() + "-" + parentSpanID.String() + "-01",
	}

	ctx, span, _ := instr.StartWithCarrier(context.Background(), "GET", carrier)
	instr.End(ctx, span, "GET", "/items/{id}", 200, time.Now())

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Parent.TraceID() != parentTraceID || spans[0].Parent.SpanID() != parentSpanID {
		t.Errorf("parent = %v/%v, want %v/%v", spans[0].Parent.TraceID(), spans[0].Parent.SpanID(), parentTraceID, parentSpanID)
	}
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

func staticRoute(route string) RouteFunc {
	return func(*http.Request) string { return route }
}

func TestMiddleware_RecordsSpanWithRouteAndStatus(t *testing.T) {
	exp := setTracer(t)
	instr := New()

	handler := instr.Middleware(staticRoute("/users/{id}"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/users/42", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name != "POST /users/{id}" {
		t.Errorf("span name = %q, want %q", span.Name, "POST /users/{id}")
	}
	var gotRoute, gotStatus bool
	for _, kv := range span.Attributes {
		if string(kv.Key) == "http.route" && kv.Value.AsString() == "/users/{id}" {
			gotRoute = true
		}
		if string(kv.Key) == "http.response.status_code" && kv.Value.AsInt64() == http.StatusCreated {
			gotStatus = true
		}
	}
	if !gotRoute {
		t.Error("missing http.route attribute")
	}
	if !gotStatus {
		t.Error("missing http.response.status_code attribute")
	}
}

func TestMiddleware_RecoversPanicAndWrites500(t *testing.T) {
	exp := setTracer(t)
	instr := New()

	handler := instr.Middleware(staticRoute("/boom"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()

	// Middleware must recover internally - ServeHTTP must not panic out to
	// the caller.
	handler.ServeHTTP(rec, req)

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
	if len(spans[0].Events) == 0 {
		t.Error("expected a recorded exception event on the span")
	}
}

func TestMiddleware_ExtractsIncomingTraceContext(t *testing.T) {
	exp := setTracer(t)
	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(prevProp) })

	instr := New()
	handler := instr.Middleware(staticRoute("/health"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	parentTraceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	parentSpanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("traceparent", "00-"+parentTraceID.String()+"-"+parentSpanID.String()+"-01")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	got := spans[0]
	if got.Parent.TraceID() != parentTraceID {
		t.Errorf("parent trace ID = %v, want %v (server span should be a child of the incoming request's trace)", got.Parent.TraceID(), parentTraceID)
	}
	if got.Parent.SpanID() != parentSpanID {
		t.Errorf("parent span ID = %v, want %v", got.Parent.SpanID(), parentSpanID)
	}
}

func TestMiddleware_DoesNotOverwriteHandlerWrittenStatus(t *testing.T) {
	setTracer(t)
	instr := New()

	handler := instr.Middleware(staticRoute("/boom"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		panic("kaboom after partial write")
	}))

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d (already written before the panic)", rec.Code, http.StatusTeapot)
	}
}
