package argosredis_test

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argosredis "github.com/jhonsferg/argos/integrations/redis"
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

// newTestClient returns a client already warmed up: go-redis v9 issues a
// HELLO/CLIENT handshake pipeline on its first command, and that would
// otherwise show up as unrelated spans in the very next test assertion.
func newTestClient(t *testing.T, opts ...argosredis.Option) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	client.AddHook(argosredis.NewHook(opts...))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("warm-up Ping: %v", err)
	}
	return client
}

func TestHook_RecordsCommandSpan(t *testing.T) {
	exp := setTracer(t)
	client := newTestClient(t)
	exp.Reset()
	ctx := context.Background()

	if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "set" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "set")
	}
	var hasSystem bool
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.system" && kv.Value.AsString() == "redis" {
			hasSystem = true
		}
	}
	if !hasSystem {
		t.Error("missing db.system=redis attribute")
	}
}

func TestHook_NilReplyIsNotAnError(t *testing.T) {
	exp := setTracer(t)
	client := newTestClient(t)
	exp.Reset()
	ctx := context.Background()

	if err := client.Get(ctx, "missing").Err(); err == nil {
		t.Fatal("expected redis.Nil for a missing key")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code == codes.Error {
		t.Error("redis.Nil must not be recorded as a span error")
	}
}

func TestHook_Pipeline(t *testing.T) {
	exp := setTracer(t)
	client := newTestClient(t)
	exp.Reset()
	ctx := context.Background()

	pipe := client.Pipeline()
	pipe.Set(ctx, "a", "1", 0)
	pipe.Set(ctx, "b", "2", 0)
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("pipeline Exec: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span for the whole pipeline, got %d", len(spans))
	}
	if spans[0].Name != "pipeline" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "pipeline")
	}
	var batchSize int64
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.operation.batch.size" {
			batchSize = kv.Value.AsInt64()
		}
	}
	if batchSize != 2 {
		t.Errorf("db.operation.batch.size = %d, want 2", batchSize)
	}
}

func TestHook_QueryTextOptIn(t *testing.T) {
	exp := setTracer(t)
	client := newTestClient(t, argosredis.WithQueryText(true))
	exp.Reset()
	ctx := context.Background()

	if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			found = true
		}
	}
	if !found {
		t.Error("expected db.query.text attribute when WithQueryText(true) is set")
	}
}

func TestHook_MaskedQueryTextOptIn(t *testing.T) {
	exp := setTracer(t)
	client := newTestClient(t, argosredis.WithMaskedQueryText(true))
	exp.Reset()
	ctx := context.Background()

	if err := client.Set(ctx, "session:abc123", "top-secret-token", 0).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

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
	if !strings.HasPrefix(got, "set ") && !strings.HasPrefix(got, "SET ") {
		t.Errorf("db.query.text = %q, want the command name kept", got)
	}
	if strings.Contains(got, "session:abc123") || strings.Contains(got, "top-secret-token") {
		t.Errorf("db.query.text = %q, still contains argument values", got)
	}
}

func TestHook_MaskedQueryText_TakesPrecedenceOverRaw(t *testing.T) {
	exp := setTracer(t)
	client := newTestClient(t, argosredis.WithQueryText(true), argosredis.WithMaskedQueryText(true))
	exp.Reset()
	ctx := context.Background()

	if err := client.Set(ctx, "k", "top-secret-token", 0).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" && strings.Contains(kv.Value.AsString(), "top-secret-token") {
			t.Errorf("db.query.text = %q, want masked even with WithQueryText also enabled", kv.Value.AsString())
		}
	}
}
