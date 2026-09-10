//go:build integration

package argosmongo_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argosmongo "github.com/jhonsferg/argos/integrations/mongo"
)

// TestMonitor_RealMongo proves the monitor against an actual MongoDB
// server, not just the hand-built events unit tests use.
func TestMonitor_RealMongo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := mongodb.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("start mongodb container: %v", err)
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

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	client, err := mongo.Connect(options.Client().ApplyURI(connStr).SetMonitor(argosmongo.NewMonitor()))
	if err != nil {
		t.Fatalf("mongo.Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	coll := client.Database("argos").Collection("items")
	if _, err := coll.InsertOne(ctx, bson.M{"name": "widget"}); err != nil {
		t.Fatalf("InsertOne: %v", err)
	}
	if err := coll.FindOne(ctx, bson.M{"name": "widget"}).Err(); err != nil {
		t.Fatalf("FindOne: %v", err)
	}

	var names []string
	for _, span := range exp.GetSpans() {
		names = append(names, span.Name)
	}
	if len(names) < 2 {
		t.Fatalf("expected at least 2 spans (insert, find), got %v", names)
	}
	var hasInsert, hasFind bool
	for _, n := range names {
		if n == "insert" {
			hasInsert = true
		}
		if n == "find" {
			hasFind = true
		}
	}
	if !hasInsert || !hasFind {
		t.Errorf("spans = %v, want to include insert and find", names)
	}
}
