package argostest

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"

	"github.com/jhonsferg/argos/logging"
)

func TestNewTracer_InstallsAndRestores(t *testing.T) {
	prev := otel.GetTracerProvider()

	t.Run("subtest", func(t *testing.T) {
		NewTracer(t)
		if otel.GetTracerProvider() == prev {
			t.Error("NewTracer did not install a new TracerProvider")
		}
	})

	if otel.GetTracerProvider() != prev {
		t.Error("NewTracer did not restore the previous TracerProvider after the subtest's cleanup ran")
	}
}

func TestNewTracer_RecordsSpans(t *testing.T) {
	exp := NewTracer(t)

	_, span := otel.Tracer("test").Start(context.Background(), "op")
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "op" {
		t.Fatalf("spans = %+v, want 1 span named %q", spans, "op")
	}
}

func TestNewLogger_RecordsAllCalls(t *testing.T) {
	rec := NewLogger(t)

	ctx := context.Background()
	rec.Debug(ctx, "d")
	rec.Info(ctx, "i", logging.F("k", "v"))
	rec.Warn(ctx, "w")
	wantErr := errors.New("boom")
	rec.Error(ctx, "e", wantErr)

	entries := rec.Entries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(entries), entries)
	}
	if entries[1].Fields[0].Key != "k" || entries[1].Fields[0].Value != "v" {
		t.Errorf("entries[1].Fields = %+v, want [k:v]", entries[1].Fields)
	}
	if entries[3].Err != wantErr {
		t.Errorf("entries[3].Err = %v, want %v", entries[3].Err, wantErr)
	}
}

func TestNewLogger_WithSharesEntries(t *testing.T) {
	rec := NewLogger(t)

	scoped := rec.With(logging.F("component", "test"))
	scoped.Info(context.Background(), "hi")

	entries := rec.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected the scoped logger's entry to appear on the parent, got %d entries", len(entries))
	}
	if entries[0].Fields[0].Key != "component" {
		t.Errorf("entries[0].Fields = %+v, want [component:test]", entries[0].Fields)
	}
}
