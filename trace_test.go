package argos

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"

	"github.com/jhonsferg/argos/argostest"
)

func TestTrace_Success(t *testing.T) {
	exp := argostest.NewTracer(t)
	rec := argostest.NewLogger(t)

	got, err := Trace(context.Background(), "orders.FindByID", func(ctx context.Context) (string, error) {
		return "order-1", nil
	})
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	if got != "order-1" {
		t.Errorf("result = %q, want %q", got, "order-1")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "orders.FindByID" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "orders.FindByID")
	}
	if spans[0].Status.Code == codes.Error {
		t.Error("span status must not be Error on success")
	}
	if len(rec.Entries()) != 0 {
		t.Errorf("expected no log entries on success, got %v", rec.Entries())
	}
}

func TestTrace_Error(t *testing.T) {
	exp := argostest.NewTracer(t)
	rec := argostest.NewLogger(t)

	wantErr := errors.New("boom")
	_, err := Trace(context.Background(), "orders.FindByID", func(ctx context.Context) (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want Error", spans[0].Status.Code)
	}
	if len(spans[0].Events) == 0 {
		t.Error("expected span.RecordError to add an event")
	}

	entries := rec.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Level != "error" || entries[0].Err != wantErr {
		t.Errorf("entry = %+v, want level=error err=%v", entries[0], wantErr)
	}
}

func TestTraceFunc(t *testing.T) {
	argostest.NewTracer(t)

	called := false
	err := TraceFunc(context.Background(), "orders.Delete", func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("TraceFunc: %v", err)
	}
	if !called {
		t.Error("fn was not called")
	}
}
