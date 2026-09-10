package propagation

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel/baggage"
	otelpropagation "go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestRoundTrip(t *testing.T) {
	p := New()

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	member, err := baggage.NewMember("k", "v")
	if err != nil {
		t.Fatalf("NewMember: %v", err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatalf("baggage.New: %v", err)
	}
	ctx = baggage.ContextWithBaggage(ctx, bag)

	carrier := otelpropagation.HeaderCarrier(http.Header{})
	p.Inject(ctx, carrier)

	gotCtx := p.Extract(context.Background(), carrier)
	gotSC := trace.SpanContextFromContext(gotCtx)
	if gotSC.TraceID() != sc.TraceID() || gotSC.SpanID() != sc.SpanID() {
		t.Fatalf("trace context did not round-trip: got %+v, want %+v", gotSC, sc)
	}

	gotBag := baggage.FromContext(gotCtx)
	if gotBag.Member("k").Value() != "v" {
		t.Fatalf("baggage did not round-trip: got %q, want %q", gotBag.Member("k").Value(), "v")
	}
}
