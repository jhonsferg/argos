// Command catalog-service is a runnable Argos sample: a gin REST API backed
// by PostgreSQL via GORM, publishing a Kafka event whenever an item is
// created. It demonstrates argosgin (server middleware), argosgorm (ORM
// plugin, itself layered on argossql), and argoskafka (producer wrapper)
// wired together under one argos.Init. See README.md for how to run it.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
	_ "github.com/jackc/pgx/v5/stdlib"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	argos "github.com/jhonsferg/argos"
	argosgorm "github.com/jhonsferg/argos/integrations/gorm"
	argossql "github.com/jhonsferg/argos/integrations/sql"
)

func main() {
	ctx := context.Background()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("catalog-service"),
		argos.WithServiceVersion("0.1.0"),
		argos.WithEnvironment("local"),
	)
	if err != nil {
		log.Fatalf("argos.Init: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			log.Printf("argos.Shutdown: %v", err)
		}
	}()
	logger := provider.Logger()

	dsn := envOr("POSTGRES_DSN", "postgres://catalog:catalog@localhost:5432/catalog?sslmode=disable")
	sqlDB, err := argossql.Open("pgx", dsn, argossql.WithSystem(semconv.DBSystemPostgreSQL))
	if err != nil {
		log.Fatalf("argossql.Open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		log.Fatalf("gorm.Open: %v", err)
	}
	if err := db.Use(argosgorm.New(argosgorm.WithSystem(semconv.DBSystemPostgreSQL))); err != nil {
		log.Fatalf("db.Use: %v", err)
	}

	store := NewItemStore(db)
	if err := store.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	kafkaCfg := sarama.NewConfig()
	// Headers (needed for trace propagation) require the message format
	// introduced in the produce/fetch v3 protocol; sarama's default Version
	// predates that and silently drops them without this.
	kafkaCfg.Version = sarama.V2_8_0_0
	kafkaCfg.Producer.Return.Successes = true
	kafkaCfg.Producer.Return.Errors = true
	producer, err := sarama.NewSyncProducer([]string{envOr("KAFKA_BROKER", "localhost:9092")}, kafkaCfg)
	if err != nil {
		log.Fatalf("sarama.NewSyncProducer: %v", err)
	}
	defer func() { _ = producer.Close() }()

	publisher := NewEventPublisher(producer)
	api := NewAPI(store, publisher, logger)

	addr := envOr("HTTP_ADDR", ":8081")
	logger.Info(ctx, "catalog-service listening", argos.F("addr", addr))
	if err := api.Routes().Run(addr); err != nil {
		log.Fatalf("gin Run: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
