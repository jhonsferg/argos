//go:build integration

package argosredis_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argosredis "github.com/jhonsferg/argos/integrations/redis"
)

// TestHook_RealRedis proves the hook against an actual Redis server, not
// just miniredis - the F2 exit criterion.
func TestHook_RealRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	opts, err := redis.ParseURL(connStr)
	if err != nil {
		t.Fatalf("parse redis URL: %v", err)
	}

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	client := redis.NewClient(opts)
	client.AddHook(argosredis.NewHook())
	defer func() { _ = client.Close() }()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("warm-up Ping: %v", err)
	}
	exp.Reset()

	if err := client.Set(ctx, "widget", "gizmo", 0).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := client.Get(ctx, "widget").Result()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "gizmo" {
		t.Errorf("Get result = %q, want %q", got, "gizmo")
	}

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans (set, get), got %d", len(spans))
	}
	if spans[0].Name != "set" || spans[1].Name != "get" {
		t.Errorf("span names = %q, %q; want set, get", spans[0].Name, spans[1].Name)
	}
}
