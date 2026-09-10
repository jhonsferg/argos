//go:build integration

package argoscassandra_test

import (
	"context"
	"testing"
	"time"

	"github.com/gocql/gocql"
	tccassandra "github.com/testcontainers/testcontainers-go/modules/cassandra"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argoscassandra "github.com/jhonsferg/argos/integrations/cassandra"
)

// TestObserver_RealCassandra proves the observer against an actual
// Cassandra server, not just the hand-built ObservedQuery/ObservedBatch
// values unit tests use.
func TestObserver_RealCassandra(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := tccassandra.Run(ctx, "cassandra:5.0")
	if err != nil {
		t.Fatalf("start cassandra container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	host, err := container.ConnectionHost(ctx)
	if err != nil {
		t.Fatalf("ConnectionHost: %v", err)
	}

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	obs := argoscassandra.New()

	setupCluster := gocql.NewCluster(host)
	setupCluster.ConnectTimeout = time.Minute
	setupSession, err := setupCluster.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession (setup): %v", err)
	}
	if err := setupSession.Query(`CREATE KEYSPACE IF NOT EXISTS argos WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`).Exec(); err != nil {
		t.Fatalf("create keyspace: %v", err)
	}
	setupSession.Close()

	cluster := gocql.NewCluster(host)
	cluster.Keyspace = "argos"
	cluster.QueryObserver = obs
	cluster.BatchObserver = obs
	session, err := cluster.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	defer session.Close()

	if err := session.Query(`CREATE TABLE IF NOT EXISTS items (id int PRIMARY KEY, name text)`).WithContext(ctx).Exec(); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := session.Query(`INSERT INTO items (id, name) VALUES (?, ?)`, 1, "widget").WithContext(ctx).Exec(); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var name string
	if err := session.Query(`SELECT name FROM items WHERE id = ?`, 1).WithContext(ctx).Scan(&name); err != nil {
		t.Fatalf("select: %v", err)
	}
	if name != "widget" {
		t.Errorf("name = %q, want %q", name, "widget")
	}

	var names []string
	for _, span := range exp.GetSpans() {
		names = append(names, span.Name)
	}
	var hasInsert, hasSelect bool
	for _, n := range names {
		if n == "INSERT" {
			hasInsert = true
		}
		if n == "SELECT" {
			hasSelect = true
		}
	}
	if !hasInsert || !hasSelect {
		t.Errorf("spans = %v, want to include INSERT and SELECT", names)
	}
}
