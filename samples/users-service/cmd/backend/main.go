// Command backend is the gRPC half of the users-service sample: an
// in-memory user store served over gRPC, instrumented with
// argosgrpc.UnaryServerInterceptor. See ../../README.md.
package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	argos "github.com/jhonsferg/argos"
	argosgrpc "github.com/jhonsferg/argos/integrations/grpc"
	"github.com/jhonsferg/argos/samples/users-service/internal/server"
	"github.com/jhonsferg/argos/samples/users-service/internal/store"
	"github.com/jhonsferg/argos/samples/users-service/userspb"
)

func main() {
	ctx := context.Background()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("users-grpc-backend"),
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

	addr := envOr("GRPC_ADDR", ":9090")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("net.Listen: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(argosgrpc.UnaryServerInterceptor()))
	userspb.RegisterUsersServer(grpcServer, server.New(store.New()))

	logger.Info(ctx, "users-grpc-backend listening", argos.F("addr", addr))
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Serve: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
