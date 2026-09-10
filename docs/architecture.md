# Architecture

## Workspace layout

Argos is a Go workspace (`go.work`), not a single module. Every integration
that pulls in a third-party SDK is its own module with its own `go.mod`, so
a consumer who only uses Redis never downloads Kafka's client as a
transitive dependency. `core` has zero dependencies on any vendor SDK.

```
argos/
├── go.work
├── core/                    Init/Shutdown/Run, Config (functional options +
│                            YAML), Trace/TraceFunc, Resource, Propagation,
│                            Logger interface + zerolog adapter, core/log
│                            (global ambient logger access), core/argostest
│                            (test fixtures), panic recovery - no vendor SDK
│                            dependencies.
├── integrations/
│   ├── httpserver/          One shared route/status-extraction core, six
│   │                        thin per-router adapters (net/http, chi, gin,
│   │                        echo, fiber, gorilla/mux).
│   ├── httpclient/          RoundTripper wrapper.
│   ├── sql/ gorm/ redis/ mongo/ cassandra/
│   ├── kafka/ rabbitmq/ azuresb/ gcppubsub/  (+ messaging/core, shared
│   │                        producer/consumer span logic + trace
│   │                        propagation carriers)
│   ├── grpc/ sftp/ smtp/
│   └── contrib/             A worked template + decision-tree guide for
│                            writing a new integration, held to the same
│                            test/benchmark bar as every real one.
├── cmd/doctor/              Scans a project's imports, suggests missing
│                            integrations.
├── samples/                 Eight runnable microservices - see Samples.
├── examples/                One minimal runnable smoke-test per phase of
│                            this library's own development.
└── docker/                  A full Grafana LGTM stack (Loki, Grafana, Tempo,
                             Mimir) plus Jaeger as a second trace backend,
                             fed by the OTel Collector for local trace/log/
                             metric inspection, plus every backend system
                             argos has an integration for (Postgres, Redis,
                             MongoDB, Cassandra, Kafka, RabbitMQ, a GCP
                             Pub/Sub emulator), independent of any sample.
```

## The Instrumentor pattern

Nearly every integration follows the same shape: a small `config`/`Option`
struct built once via functional options (`WithLogger`, `WithSystem`,
`WithQueryText`, ...), holding a lazily-created `metric.Float64Histogram`
recorded once per operation regardless of outcome, plus `Start`/`End` (or
`StartProducer`/`StartConsumer`) methods around a
`trace.WithSpanKind(...)` span. `httpserver/core` and `messaging/core`
concretely share this logic across their respective adapters; every other
module repeats the same _shape_ without literally importing shared code,
since each wraps a different vendor SDK with a different API surface.

Every module's `logger` field defaults to `core/log`'s current global
Logger when `WithLogger` isn't passed - see
[Logging](guides/logging.md#global-ambient-logging) - so error-level
logging works out of the box without every caller wiring a Logger through
every constructor.

## Wrapping a third-party SDK: three shapes

1. **The SDK exposes an interface, methods take `context.Context`.** Wrap
   the interface directly - see `integrations/contrib/example.go`, the
   simplest case.
2. **A concrete struct with no `context.Context` parameter, no
   cross-process propagation.** Two variants: embed the struct and add
   `*Context`-suffixed sibling methods for the operations worth a span
   (`integrations/sftp`, since Go's method promotion keeps every
   unoverridden method working); or, if there's no single type to embed at
   all (a package-level function, or a producer type with no context
   parameter), wrap it as a free function (`integrations/smtp`,
   `integrations/kafka`, `integrations/rabbitmq`).
3. **Message-passing where trace context itself must travel with the
   message.** A small `propagation.TextMapCarrier` adapter over whatever the
   library uses for message headers/properties, so
   `otel.GetTextMapPropagator().Inject`/`Extract` can read and write it -
   see `messaging/core` and its four consumers (Kafka, RabbitMQ, Azure
   Service Bus, GCP Pub/Sub).

## Zero-allocation discipline

Every integration is designed assuming it runs in the hot path of the
highest-QPS request in a service:

- `sync.Pool` for span attribute buffers, log serialization buffers, and
  reusable intermediate context structs, where the module's own benchmarks
  show it matters.
- Pre-sized attribute slices instead of unbounded `append`.
- No `interface{}`/`any` or reflection on the hot path - reflection is
  confined to configuration-time code.
- Static attributes (service name, environment, ...) computed once in
  `Init()`, never per request.
- No `fmt.Sprintf` building span names or attributes on the hot path.

Every module ships a comparative benchmark
(`Benchmark..._Baseline` vs. `Benchmark..._Instrumented`, `-benchmem`)
documenting the allocation cost the wrapper adds - a handful of allocations
per call for the span/metric pipeline itself is normal and expected; the
discipline is about not adding _avoidable_ ones on top of that.

## Fail-open telemetry

`argos.Init` never fails solely because the configured OTLP collector is
unreachable - the SDK's own OTLP exporters retry and drop in the
background. A service instrumented with Argos keeps serving requests with
zero telemetry infrastructure running; wiring up a collector is additive,
never a prerequisite.
