//go:build integration

package argosgorm_test

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
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	argosgorm "github.com/jhonsferg/argos/integrations/gorm"
	argossql "github.com/jhonsferg/argos/integrations/sql"
)

// TestPlugin_UsesArgosSQLUnderneath proves the "argos-gorm uses argos-sql
// underneath" composition against a real PostgreSQL server: the underlying
// *sql.DB is opened through argossql.Open, and GORM is handed that
// connection directly, so both layers' spans should appear for one query -
// this is the F2 exit criterion for this module.
func TestPlugin_UsesArgosSQLUnderneath(t *testing.T) {
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

	sqlDB, err := argossql.Open("pgx", dsn, argossql.WithSystem(semconv.DBSystemPostgreSQL))
	if err != nil {
		t.Fatalf("argossql.Open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	if err := db.Use(argosgorm.New(argosgorm.WithSystem(semconv.DBSystemPostgreSQL))); err != nil {
		t.Fatalf("db.Use: %v", err)
	}

	type widget struct {
		ID   uint
		Name string
	}
	if err := db.WithContext(ctx).AutoMigrate(&widget{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	exp.Reset()

	if err := db.WithContext(ctx).Create(&widget{Name: "gizmo"}).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	var scopes, drivers int
	for _, span := range exp.GetSpans() {
		switch span.InstrumentationScope.Name {
		case "github.com/jhonsferg/argos/integrations/gorm":
			scopes++
		case "github.com/jhonsferg/argos/integrations/sql":
			drivers++
		}
	}
	if scopes == 0 {
		t.Error("expected at least one span from argos-gorm")
	}
	if drivers == 0 {
		t.Error("expected at least one span from argos-sql - the underlying driver should be instrumented too")
	}
}
