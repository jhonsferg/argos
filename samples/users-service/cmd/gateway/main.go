// Command gateway is the HTTP half of the users-service sample: a
// gorilla/mux router translating GET/POST /users to gRPC calls against the
// backend, instrumented with argosgorillamux (server) and
// argosgrpc.UnaryClientInterceptor (client) - the same trace propagates
// from the incoming HTTP request through to the outgoing gRPC call. See
// ../../README.md.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	argos "github.com/jhonsferg/argos"
	argosgrpc "github.com/jhonsferg/argos/integrations/grpc"
	"github.com/jhonsferg/argos/samples/users-service/userspb"
)

func main() {
	ctx := context.Background()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("users-gateway"),
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

	conn, err := grpc.NewClient(envOr("BACKEND_ADDR", "localhost:9090"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(argosgrpc.UnaryClientInterceptor()),
	)
	if err != nil {
		log.Fatalf("grpc.NewClient: %v", err)
	}
	defer func() { _ = conn.Close() }()

	api := NewAPI(userspb.NewUsersClient(conn), logger)

	addr := envOr("HTTP_ADDR", ":8085")
	logger.Info(ctx, "users-gateway listening", argos.F("addr", addr))
	srv := &http.Server{Addr: addr, Handler: api.Routes(), ReadHeaderTimeout: 5 * time.Second}
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
