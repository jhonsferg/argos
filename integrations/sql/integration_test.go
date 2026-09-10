//go:build integration

package argossql_test

import (
	"context"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	argossql "github.com/jhonsferg/argos/integrations/sql"
)

// TestOpen_RealPostgres proves the wrapper against an actual PostgreSQL
// server, not just the fakes unit tests use - the F2 exit criterion.
func TestOpen_RealPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("argos"),
		postgres.WithUsername("argos"),
		postgres.WithPassword("argos"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	db, err := argossql.Open("pgx", dsn, argossql.WithSystem(semconv.DBSystemPostgreSQL))
	if err != nil {
		t.Fatalf("argossql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(ctx, "CREATE TABLE items (id SERIAL PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "widget"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	rows, err := db.QueryContext(ctx, "SELECT name FROM items WHERE id = $1", 1)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !rows.Next() {
		t.Fatal("expected 1 row")
	}
	var name string
	if err := rows.Scan(&name); err != nil {
		t.Fatalf("scan: %v", err)
	}
	_ = rows.Close()
	if name != "widget" {
		t.Errorf("name = %q, want %q", name, "widget")
	}

	spans := exp.GetSpans()
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans (create table, insert, query), got %d", len(spans))
	}
	for _, span := range spans {
		var hasSystem bool
		for _, kv := range span.Attributes {
			if string(kv.Key) == "db.system" && kv.Value.AsString() == "postgresql" {
				hasSystem = true
			}
		}
		if !hasSystem {
			t.Errorf("span %q missing db.system=postgresql attribute", span.Name)
		}
	}
}
