// Command core-dummy is the minimal consumer of argos-core that proves
// go.work wires the workspace's local modules together end-to-end - no
// replace directives needed.
package main

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel"

	argos "github.com/jhonsferg/argos"
)

func main() {
	ctx := context.Background()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("core-dummy"),
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

	tracer := otel.Tracer("core-dummy")
	ctx, span := tracer.Start(ctx, "do-work")
	provider.Logger().Info(ctx, "hello from core-dummy", argos.F("attempt", 1))
	span.End()
}
