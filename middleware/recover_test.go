package middleware

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHandle_RecordsErrorOnSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "op")
	err := Handle(ctx, nil, "boom")
	span.End()

	if err == nil {
		t.Fatal("expected non-nil error for a recovered panic")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	got := spans[0]
	if got.Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", got.Status.Code, codes.Error)
	}
	if len(got.Events) == 0 {
		t.Fatal("expected an exception event on the span")
	}
}

func TestHandle_NilRecoveredIsNoop(t *testing.T) {
	if err := Handle(context.Background(), nil, nil); err != nil {
		t.Fatalf("expected nil error for nil recovered value, got %v", err)
	}
}
