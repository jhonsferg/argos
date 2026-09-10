package argosmongo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argosmongo "github.com/jhonsferg/argos/integrations/mongo"
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

// TestMonitor_StartedThenSucceeded drives the monitor exactly the way the
// real driver does: Started, then Succeeded, correlated by RequestID - no
// live server needed since these are just plain callback funcs.
func TestMonitor_StartedThenSucceeded(t *testing.T) {
	exp := setTracer(t)
	mon := argosmongo.NewMonitor()

	ctx := context.Background()
	mon.Started(ctx, &event.CommandStartedEvent{
		CommandName:  "find",
		DatabaseName: "orders",
		RequestID:    1,
	})
	mon.Succeeded(ctx, &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName:  "find",
			DatabaseName: "orders",
			RequestID:    1,
		},
	})

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "find" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "find")
	}
	var hasNamespace bool
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.namespace" && kv.Value.AsString() == "orders" {
			hasNamespace = true
		}
	}
	if !hasNamespace {
		t.Error("missing db.namespace=orders attribute")
	}
}

func TestMonitor_QueryTextOptIn(t *testing.T) {
	exp := setTracer(t)
	mon := argosmongo.NewMonitor(argosmongo.WithQueryText(true))

	cmd, err := bson.Marshal(bson.D{{Key: "insert", Value: "orders"}, {Key: "documents", Value: bson.A{bson.D{{Key: "email", Value: "user@example.com"}}}}})
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}

	ctx := context.Background()
	mon.Started(ctx, &event.CommandStartedEvent{CommandName: "insert", DatabaseName: "orders", RequestID: 3, Command: bson.Raw(cmd)})
	mon.Succeeded(ctx, &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{CommandName: "insert", DatabaseName: "orders", RequestID: 3},
	})

	var got string
	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			got, found = kv.Value.AsString(), true
		}
	}
	if !found {
		t.Fatal("expected db.query.text attribute when WithQueryText(true) is set")
	}
	if !strings.Contains(got, "user@example.com") {
		t.Errorf("db.query.text = %q, want the raw command including its values", got)
	}
}

func TestMonitor_MaskedQueryTextOptIn(t *testing.T) {
	exp := setTracer(t)
	mon := argosmongo.NewMonitor(argosmongo.WithMaskedQueryText(true))

	cmd, err := bson.Marshal(bson.D{{Key: "insert", Value: "orders"}, {Key: "documents", Value: bson.A{bson.D{{Key: "email", Value: "user@example.com"}}}}})
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}

	ctx := context.Background()
	mon.Started(ctx, &event.CommandStartedEvent{CommandName: "insert", DatabaseName: "orders", RequestID: 4, Command: bson.Raw(cmd)})
	mon.Succeeded(ctx, &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{CommandName: "insert", DatabaseName: "orders", RequestID: 4},
	})

	var got string
	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			got, found = kv.Value.AsString(), true
		}
	}
	if !found {
		t.Fatal("expected db.query.text attribute when WithMaskedQueryText(true) is set")
	}
	if strings.Contains(got, "user@example.com") {
		t.Errorf("db.query.text = %q, still contains a field value", got)
	}
	if !strings.Contains(got, "insert") || !strings.Contains(got, "documents") {
		t.Errorf("db.query.text = %q, want top-level field names kept", got)
	}
}

func TestMonitor_Failed_RecordsError(t *testing.T) {
	exp := setTracer(t)
	mon := argosmongo.NewMonitor()

	ctx := context.Background()
	mon.Started(ctx, &event.CommandStartedEvent{CommandName: "insert", DatabaseName: "orders", RequestID: 2})
	mon.Failed(ctx, &event.CommandFailedEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{CommandName: "insert", DatabaseName: "orders", RequestID: 2},
		Failure:              errors.New("duplicate key"),
	})

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}

func TestMonitor_UnknownRequestIDIsIgnored(t *testing.T) {
	exp := setTracer(t)
	mon := argosmongo.NewMonitor()

	// Succeeded with no matching Started (e.g. missed due to sampling
	// upstream) must not panic and must not fabricate a span.
	mon.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{CommandName: "find", RequestID: 999},
	})

	if spans := exp.GetSpans(); len(spans) != 0 {
		t.Fatalf("expected 0 spans, got %d", len(spans))
	}
}
