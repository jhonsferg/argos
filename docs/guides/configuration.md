# Configuration

Every `argos.Init` option can be set from Go code, from a YAML file, or a mix
of both - whichever fits how a given service manages its configuration.

## Functional options

The baseline, used throughout the other guides:

```go
provider, err := argos.Init(ctx,
	argos.WithServiceName("orders-api"),
	argos.WithServiceVersion("1.4.0"),
	argos.WithEnvironment("production"),
	argos.WithOTLPEndpoint("otel-collector:4317"),
	argos.WithSampleRatio(0.25),
)
```

## Declarative YAML config

`argos.WithYAMLConfig(path)` loads a YAML file and layers it directly into
the option chain - it composes with whatever came before or after it,
instead of replacing the whole config:

```go
provider, err := argos.Init(ctx,
	argos.WithYAMLConfig("config.yaml"),
	argos.WithServiceVersion(buildVersion), // still overrides the YAML value
)
```

```yaml
# config.yaml
service_name: orders-api
service_version: 1.4.0
environment: production

otlp_endpoint: otel-collector:4317
otlp_protocol: grpc # grpc | http
otlp_insecure: true

sample_ratio: 0.25
```

Options applied **after** `WithYAMLConfig` in the chain override whatever
the file set for that field; options applied **before** it are themselves
overridden by the file, the same layering order `argos.Init` already uses
for environment variables (`OTEL_SERVICE_NAME` and friends) versus
functional options. If the file can't be read or parsed, `Init` returns
that error - nothing panics.

If you need to inspect or mutate the loaded config before calling `Init`,
use the lower-level two-step form instead:

```go
cfg, err := argos.FromYAML("config.yaml")
// cfg.SampleRatio, cfg.ServiceName, ... are plain fields here
provider, err := argos.Init(ctx, argos.WithConfig(cfg))
```

## Per-integration YAML sections

Some integrations expose their own options through the same YAML file,
under an `integrations:` map keyed by the integration's name. `core` never
inspects these sections itself - each integration decodes its own key on
demand via `core.DecodeIntegrationConfig`, so adding a new configurable
integration never requires a change to `core`.

`integrations/sql` supports this today:

```yaml
integrations:
  sql:
    query_text: true
```

```go
cfg, err := argos.FromYAML("config.yaml")

db, err := argossql.Open("pgx", dsn,
	argossql.WithYAMLConfig(cfg),
	argossql.WithSystem(semconv.DBSystemPostgreSQL), // functional options still layer on top
)
```

A missing `integrations.<name>` section is not an error - the integration
just keeps whatever its Go-code options (or defaults) already set. See
[`integrations/contrib`](https://github.com/jhonsferg/argos/tree/main/integrations/contrib)
for the pattern to add this to a module that doesn't have it yet.

## Local observability stack

`docker/docker-compose.otel.yml` runs a full Grafana LGTM stack - **L**oki
(logs), **G**rafana (UI), **T**empo (traces), **M**imir (metrics) - fed by
the OTel Collector, so what you see locally matches what a real deployment
reports to, not just a single trace viewer:

```bash
make otel-up      # otel-collector, tempo, jaeger, loki, mimir, grafana
```

Open Grafana at `http://localhost:3000` (`admin`/`admin`). The three
datasources are pre-provisioned and cross-linked: a metric's exemplar jumps
to its trace in Tempo, a log line's `trace_id` label jumps to the same
place, and a trace's `tracesToLogs`/`tracesToMetrics` jump the other way.
An `Argos` dashboard folder ships with one starter dashboard covering the
metrics every integration emits (`http.server.request.duration`,
`db.client.operation.duration`, `messaging.client.operation.duration`,
`operation.duration` from `argos.Trace`).

Jaeger runs alongside Tempo as a second, independent trace backend fed the
same OTLP traces (`otel-collector-config.yaml`'s traces pipeline exports to
both) - its own UI is at `http://localhost:16686`. It isn't wired into
Grafana as a datasource; use it when you specifically want Jaeger's own
trace view rather than Tempo/Grafana's.

Separately, `docker/docker-compose.systems.yml` brings up every backend
system argos has an integration for - Postgres, Redis, MongoDB, Cassandra,
Kafka, RabbitMQ, and a GCP Pub/Sub emulator (Azure Service Bus excluded, no
viable local emulator) - independent of any one sample, for manual
exploration or for a sample to depend on:

```bash
make systems-up   # everything above, make systems-down to tear it down
```

`samples/inventory-api` and `samples/inventory-worker` are built against
this stack and are the best way to see it in action: one HTTP request
there produces a single trace spanning both processes across five
different integrations.

## Sampling: head vs. tail

`WithSampleRatio`/`WithSampler` are **head sampling**: the decision is made
per-trace, at the very moment its root span starts, before anything is
known about how the trace will end. This is the only kind of sampling
`argos.Init` itself can do - there is no SDK-side way to "always keep
traces that end in an error," because that outcome isn't knowable yet at
the point the SDK decides.

Keeping 100% of error traces while thinning out successful ones needs
**tail sampling** instead: a decision made after a trace completes,
downstream in the OTel Collector, which buffers each trace's spans until
it's finished before deciding whether to keep it. This repo ships a
ready-to-use recipe for it - `docker/otel-collector-tailsampling-config.yaml`,
run via:

```bash
make otel-up-tailsampling
```

It keeps 100% of traces containing an error-status span and a tunable
percentage (10% by default) of everything else - adjust
`sampling_percentage` in that file to taste. This only changes what the
_collector_ forwards/retains; `argos.Init`'s own `WithSampleRatio` still
decides what leaves the service at all, so a low head sample ratio and tail
sampling are complementary, not alternatives - head sampling controls
overall volume, tail sampling controls which of that volume is worth
keeping.

## Environment variables

Independent of both of the above, `argos.Init` always reads the standard
OTel environment variables first (`OTEL_SERVICE_NAME`,
`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`,
`OTEL_EXPORTER_OTLP_INSECURE`, `DEPLOYMENT_ENVIRONMENT`), applied before any
`Option` - including `WithYAMLConfig` - so both YAML and functional options
can still override them.
