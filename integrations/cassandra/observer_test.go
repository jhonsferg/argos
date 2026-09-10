package argoscassandra_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argoscassandra "github.com/jhonsferg/argos/integrations/cassandra"
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

func TestObserveQuery_RecordsSpan(t *testing.T) {
	exp := setTracer(t)
	obs := argoscassandra.New()

	start := time.Now()
	obs.ObserveQuery(context.Background(), gocql.ObservedQuery{
		Keyspace:  "orders",
		Statement: "SELECT * FROM items WHERE id = ?",
		Start:     start,
		End:       start.Add(5 * time.Millisecond),
	})

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "SELECT" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "SELECT")
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
	if !spans[0].StartTime.Equal(start) {
		t.Errorf("span start time = %v, want %v (retroactive timestamp)", spans[0].StartTime, start)
	}
}

func TestObserveQuery_RecordsError(t *testing.T) {
	exp := setTracer(t)
	obs := argoscassandra.New()

	now := time.Now()
	obs.ObserveQuery(context.Background(), gocql.ObservedQuery{
		Statement: "INSERT INTO items (id) VALUES (?)",
		Start:     now,
		End:       now.Add(time.Millisecond),
		Err:       errors.New("timeout"),
	})

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}

func TestObserveBatch_RecordsBatchSize(t *testing.T) {
	exp := setTracer(t)
	obs := argoscassandra.New()

	now := time.Now()
	obs.ObserveBatch(context.Background(), gocql.ObservedBatch{
		Keyspace:   "orders",
		Statements: []string{"INSERT INTO items (id) VALUES (?)", "INSERT INTO items (id) VALUES (?)"},
		Start:      now,
		End:        now.Add(2 * time.Millisecond),
	})

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "batch" {
		t.Fatalf("expected 1 span named batch, got %v", spans)
	}
	var batchSize int64
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "messaging.batch.message_count" {
			batchSize = kv.Value.AsInt64()
		}
	}
	if batchSize != 2 {
		t.Errorf("messaging.batch.message_count = %d, want 2", batchSize)
	}
}

func TestQueryTextOptIn(t *testing.T) {
	exp := setTracer(t)
	obs := argoscassandra.New(argoscassandra.WithQueryText(true))

	now := time.Now()
	obs.ObserveQuery(context.Background(), gocql.ObservedQuery{
		Statement: "SELECT * FROM accounts WHERE id = 5 AND name = 'bob'",
		Start:     now,
		End:       now,
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
	if !strings.Contains(got, "bob") {
		t.Errorf("db.query.text = %q, want the raw statement including its literals", got)
	}
}

func TestMaskedQueryTextOptIn(t *testing.T) {
	exp := setTracer(t)
	obs := argoscassandra.New(argoscassandra.WithMaskedQueryText(true))

	now := time.Now()
	obs.ObserveQuery(context.Background(), gocql.ObservedQuery{
		Statement: "SELECT * FROM accounts WHERE id = 5 AND name = 'bob'",
		Start:     now,
		End:       now,
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
	if strings.Contains(got, "bob") || strings.Contains(got, "5") {
		t.Errorf("db.query.text = %q, still contains literal values", got)
	}
}

func TestMaskedQueryText_TakesPrecedenceOverRaw(t *testing.T) {
	exp := setTracer(t)
	obs := argoscassandra.New(argoscassandra.WithQueryText(true), argoscassandra.WithMaskedQueryText(true))

	now := time.Now()
	obs.ObserveQuery(context.Background(), gocql.ObservedQuery{
		Statement: "SELECT * FROM accounts WHERE name = 'bob'",
		Start:     now,
		End:       now,
	})

	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" && strings.Contains(kv.Value.AsString(), "bob") {
			t.Errorf("db.query.text = %q, want masked even with WithQueryText also enabled", kv.Value.AsString())
		}
	}
}

func TestQueryTextOffByDefault(t *testing.T) {
	exp := setTracer(t)
	obs := argoscassandra.New()

	now := time.Now()
	obs.ObserveQuery(context.Background(), gocql.ObservedQuery{
		Statement: "SELECT secret FROM accounts",
		Start:     now,
		End:       now,
	})

	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			t.Error("db.query.text must not be recorded unless WithQueryText(true) is set")
		}
	}
}
