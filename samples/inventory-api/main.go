// Command inventory-api is a runnable Argos sample combining several
// integrations in one request path: a net/http API that reserves stock via
// a local gRPC call (argosgrpc), backed by PostgreSQL (argossql) with a
// Redis cache-aside read layer (argosredis), publishing a domain event to
// Kafka (argoskafka) on every successful reservation. One HTTP request
// produces a trace spanning HTTP -> gRPC -> Postgres -> Kafka producer. It
// also demonstrates argos.Run (startup/shutdown/signal-handling in one
// call), argos.TraceFunc (see grpcserver.go), and core/log's global logger
// (see store.go) - no Logger instance is threaded through any constructor
// here. See README.md for how to run it, including the companion
// inventory-worker sample that consumes the Kafka event this produces.
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/IBM/sarama"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	argos "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/capture"
	argosgrpc "github.com/jhonsferg/argos/integrations/grpc"
	httpservercore "github.com/jhonsferg/argos/integrations/httpserver/core"
	argosnethttp "github.com/jhonsferg/argos/integrations/httpserver/nethttp"
	argosredis "github.com/jhonsferg/argos/integrations/redis"
	argossql "github.com/jhonsferg/argos/integrations/sql"
	"github.com/jhonsferg/argos/samples/inventory-api/inventorypb"
)

func main() {
	err := argos.Run(context.Background(), []argos.Option{
		argos.WithServiceName("inventory-api"),
		argos.WithServiceVersion("0.1.0"),
		argos.WithEnvironment("local"),
	}, run)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, provider *argos.Provider) error {
	logger := provider.Logger()

	dsn := envOr("POSTGRES_DSN", "postgres://argos:argos@localhost:5432/argos?sslmode=disable")
	db, err := argossql.Open("pgx", dsn, argossql.WithSystem(semconv.DBSystemPostgreSQL), argossql.WithMaskedQueryText(true))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	cache := redis.NewClient(&redis.Options{Addr: envOr("REDIS_ADDR", "localhost:6379")})
	cache.AddHook(argosredis.NewHook(argosredis.WithSystem(semconv.DBSystemRedis)))
	defer func() { _ = cache.Close() }()

	store := NewItemStore(db, cache)
	migrateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := store.Migrate(migrateCtx); err != nil {
		return err
	}

	kafkaCfg := sarama.NewConfig()
	kafkaCfg.Version = sarama.V2_8_0_0 // headers (trace propagation) need produce/fetch v3+
	kafkaCfg.Producer.Return.Successes = true
	kafkaCfg.Producer.Return.Errors = true
	producer, err := sarama.NewSyncProducer([]string{envOr("KAFKA_BROKER", "localhost:9092")}, kafkaCfg)
	if err != nil {
		return err
	}
	defer func() { _ = producer.Close() }()
	events := NewEventPublisher(producer)

	grpcAddr := envOr("GRPC_ADDR", ":9091")
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(argosgrpc.UnaryServerInterceptor()))
	inventorypb.RegisterInventoryServer(grpcServer, NewInventoryServer(store))
	go func() {
		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			logger.Error(ctx, "grpc Serve failed", serveErr)
		}
	}()
	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	conn, err := grpc.NewClient(envOr("GRPC_ADDR", "localhost:9091"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(argosgrpc.UnaryClientInterceptor(argosgrpc.WithCapture(capture.HTTPRules{
			RequestHeaders:  capture.HeaderRule{Enabled: true, Exclude: []string{"authorization"}},
			ResponseHeaders: capture.HeaderRule{Enabled: true},
		}))),
	)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	api := NewAPI(store, inventorypb.NewInventoryClient(conn), events)
	handler := argosnethttp.Middleware(httpservercore.WithCaptureBodyOnError(4096))(api.Routes())

	httpAddr := envOr("HTTP_ADDR", ":8086")
	logger.Info(ctx, "inventory-api listening", argos.F("http_addr", httpAddr), argos.F("grpc_addr", grpcAddr))
	srv := &http.Server{Addr: httpAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
