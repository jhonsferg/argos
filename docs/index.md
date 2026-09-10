# Argos

Zero-allocation-focused declarative OpenTelemetry instrumentation for Go
microservices - a single `Init()` that wires tracing, metrics, and
correlated logging, plus drop-in wrappers per client library.

## Why Argos

Go has no equivalent to Java's `-javaagent` bytecode instrumentation - no
classloader, no runtime weaving. The only real "zero-code" option is eBPF,
which needs elevated privileges, a recent Linux kernel, and only covers a
subset of libraries. Argos takes the other path: a **low-code** library.
One `argos.Init()` call at startup, then an explicit wrapper per client you
actually use. No monkey-patching, no hidden magic, no reflection on the hot
path.

## What's in the box

- **`core`** - `Init()`/`Shutdown()` (or `Run()` for the common case: startup,
  signal-triggered shutdown, and `Shutdown` in one call), functional-options
  or declarative-YAML configuration (`argos.WithYAMLConfig`), a generic
  `Trace[T]`/`TraceFunc` for instrumenting your own business logic without
  a struct field, Resource detection, W3C tracecontext+baggage propagation,
  a zero-allocation-conscious `Logger` interface (zerolog-backed by
  default) with automatic `trace_id`/`span_id` correlation, global ambient
  logger access (`core/log`) so nested layers can log without threading a
  Logger instance through constructors, `core/argostest` in-memory
  tracer/logger fixtures for testing code instrumented with Argos, panic
  recovery.
- **HTTP servers** - net/http, chi, gin, echo, fiber, gorilla/mux, each a
  thin adapter over one shared route/status extraction core.
- **HTTP client** - an `http.RoundTripper` wrapper (drop-in, like
  `otelhttp.NewTransport`).
- **Data layer** - a driver-agnostic `database/sql` wrapper (works with
  any driver), a GORM plugin, Redis hook, MongoDB `CommandMonitor`,
  Cassandra `QueryObserver`/`BatchObserver`.
- **Messaging** - Kafka, RabbitMQ, Azure Service Bus, GCP Pub/Sub, all
  propagating trace context from producer to consumer across the real
  broker.
- **gRPC** - unary/stream server and client interceptors.
- **Niche** - SFTP client wrapper, SMTP `SendMail` wrapper.
- **`cmd/doctor`** - a CLI that scans a project's own imports and suggests
  any Argos integration matching a vendor SDK already in use, or (`-init`)
  scaffolds a starter `argos.config.yaml` and `main.go` snippet for a new one.

Every integration is its own Go module with its own `go.mod` - importing
`argos-redis` never pulls in Kafka's client library. See
[Architecture](architecture.md) for the full workspace layout and the
zero-allocation design principles every module is held to.

## Try it

The [Samples](samples.md) page has six runnable microservices, each mixing
a different combination of router + data/messaging technology, with its own
`docker-compose.yml` and README - the fastest way to see Argos wired into
something real.
