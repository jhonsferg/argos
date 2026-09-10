# Quick Start

A minimal net/http service with tracing, metrics, and correlated logging,
using `argos.Run` to handle startup/shutdown/signal-handling in one call:

```go
package main

import (
	"context"
	"log"
	"net/http"

	argos "github.com/jhonsferg/argos"
	argoslog "github.com/jhonsferg/argos/log"
	argosnethttp "github.com/jhonsferg/argos/integrations/httpserver/nethttp"
)

func main() {
	err := argos.Run(context.Background(), []argos.Option{
		argos.WithServiceName("my-service"),
		argos.WithServiceVersion("0.1.0"),
		argos.WithEnvironment("local"),
	}, func(ctx context.Context, provider *argos.Provider) error {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
			argoslog.Info(r.Context(), "handling health check")
			w.WriteHeader(http.StatusOK)
		})

		srv := &http.Server{Addr: ":8080", Handler: argosnethttp.Middleware()(mux)}
		go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

That's the whole pattern used by every integration in this library:

1. `argos.Run(ctx, opts, serve)` once, at startup - it calls `Init`, runs
   `serve` with a context cancelled on SIGINT/SIGTERM, then always calls
   `Shutdown` once `serve` returns.
2. Wrap whatever client(s) you use - a router middleware, a driver, a hook,
   a producer/consumer call - with the matching Argos integration.
3. Log from anywhere via `core/log` (`argoslog.Info`/`Error`/...) - no
   `Logger` instance needs to reach the call site; see
   [Logging](guides/logging.md#global-ambient-logging).

## Manual Init/Shutdown

`Run` is a convenience over `Init`/`Shutdown`, not a requirement - reach for
the lower-level pair when you need more control over shutdown ordering (a
worker with no single "serve" call, or several long-running goroutines to
coordinate):

```go
provider, err := argos.Init(ctx, argos.WithServiceName("my-service"))
if err != nil {
	log.Fatalf("argos.Init: %v", err)
}
defer func() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = provider.Shutdown(shutdownCtx)
}()
```

## Instrumenting your own business logic

`argos.Trace`/`argos.TraceFunc` wrap arbitrary code in a span in one line,
without a struct field or a per-integration constructor:

```go
order, err := argos.Trace(ctx, "orders.FindByID", func(ctx context.Context) (*Order, error) {
	return repo.findByID(ctx, id)
})
```

See [Tracing custom operations](guides/tracing.md) for details, including
error/log correlation and `TraceFunc` for operations with no return value.

## Where traces/metrics go

By default, Argos exports OTLP to `localhost:4317` (gRPC, insecure) and
**fails open**: if no collector is listening, requests still work - traces
and metrics are just dropped. Logs always go to stdout as structured JSON
regardless. Point it elsewhere with:

```go
argos.WithOTLPEndpoint("otel-collector:4317")
```

or the standard `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable.

To see real traces, logs, and metrics locally, run the Grafana LGTM +
OTel Collector stack this repo ships (see
[Configuration](guides/configuration.md) for what's in it and how to bring
up the backend systems argos instruments too):

```bash
make otel-up
```

## Next steps

- Pick a [Sample](samples.md) closest to your stack and read its README.
- Browse the [Guides](guides/http-servers.md) for wiring snippets per
  integration.
- See [Configuration](guides/configuration.md) for loading this from a YAML
  file instead of (or alongside) functional options - `cmd/doctor -init`
  (see [Doctor CLI](guides/doctor-cli.md)) generates a starter one for you.
- [Testing](guides/testing.md) covers `argostest`, the in-memory
  tracer/logger fixtures for testing code instrumented with Argos.
- Read [Architecture](architecture.md) for the design principles behind the
  library (zero-allocation discipline, the Instrumentor pattern, workspace
  layout).
