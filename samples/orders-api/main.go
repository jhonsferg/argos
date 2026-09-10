// Command orders-api is a runnable Argos sample: a net/http REST API backed
// by PostgreSQL, with Redis as a cache-aside read-through layer in front of
// it. It demonstrates argosnethttp (server middleware), argossql (Postgres
// driver wrapper), and argosredis (client hook) wired together under one
// argos.Init. See README.md for how to run it.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	argos "github.com/jhonsferg/argos"
	argosnethttp "github.com/jhonsferg/argos/integrations/httpserver/nethttp"
	argosredis "github.com/jhonsferg/argos/integrations/redis"
	argossql "github.com/jhonsferg/argos/integrations/sql"
)

func main() {
	ctx := context.Background()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("orders-api"),
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

	dsn := envOr("POSTGRES_DSN", "postgres://orders:orders@localhost:5432/orders?sslmode=disable")
	db, err := argossql.Open("pgx", dsn, argossql.WithSystem(semconv.DBSystemPostgreSQL))
	if err != nil {
		log.Fatalf("argossql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	cache := redis.NewClient(&redis.Options{Addr: envOr("REDIS_ADDR", "localhost:6379")})
	cache.AddHook(argosredis.NewHook(argosredis.WithSystem(semconv.DBSystemRedis)))
	defer func() { _ = cache.Close() }()

	store := NewOrderStore(db, cache)
	migrateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := store.Migrate(migrateCtx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	api := NewAPI(store, logger)
	handler := argosnethttp.Middleware()(api.Routes())

	addr := envOr("HTTP_ADDR", ":8080")
	logger.Info(ctx, "orders-api listening", argos.F("addr", addr))
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
