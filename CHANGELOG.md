# Changelog

All notable changes to this repository are documented here, across every module,
in one chronological log - see [Repository layout](README.md#repository-layout)
for why this isn't split per module. Each module is versioned and tagged
independently (`<module-path>/vX.Y.Z`); an entry below names the specific
module(s) it applies to.

## [Initial release] - 2026-09-10

First tagged release of every module in this repository. Module versions
start above `v0.1.0` (root at `v0.1.1`, most integrations at `v0.1.2`,
`integrations/gcppubsub` at `v0.1.3`) to avoid colliding with version numbers
already recorded in the Go checksum database from an earlier iteration of
this repository's tag history.

- **Root module** (`v0.1.1`): `Init`/`Shutdown`/`Run`, functional-options or
  YAML configuration, the global ambient logger, the `Trace`/`TraceFunc`
  span-wrap helper, panic recovery middleware, and `argostest` test
  fixtures. Zero dependencies on any vendor SDK.
- **Configurable capture rules**, backed by the new `capture` package
  (`capture.MaskSQL`, `capture.ApplyHeaderRule`): `WithMaskedQueryText`
  (redacted query text) on the five database integrations, and full
  request/response body plus header capture (`WithCapture`, plus the
  `WithCaptureBodyOnError` sugar) on the HTTP client, HTTP server core, and
  gRPC integrations. Configurable via both functional options and YAML.
- `capture.MaskSQL` was fuzz-tested (see `capture/fuzz_test.go`), which found
  and fixed a real bug: an apostrophe inside a SQL comment or a double-quoted
  identifier could desynchronize the masking regex and let a real literal
  value (e.g. an email address) leak unmasked into span attributes. Numeric
  literal masking was also extended to hexadecimal and scientific notation.
  See [docs/guides/sql.md](docs/guides/sql.md) for the specific guarantees
  this uncovered and the known limitations.
- **Integrations** (`v0.1.2`, `integrations/gcppubsub` at `v0.1.3`): tracing,
  metrics, and correlated logging for SQL (any `database/sql` driver), GORM,
  Redis, MongoDB, Cassandra, Kafka, RabbitMQ, gRPC, HTTP client, six HTTP
  server routers (net/http, chi, gin, echo, fiber, gorilla/mux), SFTP, SMTP,
  GCP Pub/Sub, and Azure Service Bus - one Go module per integration, each
  with its own `go.mod` so consumers only pull in the vendor SDK they
  actually use.
- **`cmd/doctor`** (`v0.1.2`): a CLI that scans a Go project's own imports and
  suggests any Argos integration matching a vendor SDK it already uses but
  hasn't wired up.
- **Samples**: eight runnable microservices verified end to end against real
  Docker infrastructure, covering every integration above in realistic
  combinations. Samples and examples are reference code and are never tagged
  or released as versioned modules.
- Real per-module test coverage was measured (not just pass/fail) and the
  highest-risk gaps closed: `integrations/sql`'s entire legacy
  `database/sql/driver` fallback path (the reason the package exists, per its
  own doc comment) had zero coverage; `WithLogger` was untested in every
  single DB/messaging integration; `integrations/httpserver/core`'s
  streaming/hijacking compatibility layer (`Flush`/`Unwrap`) was untested.
- Ran a 1-hour sustained-load soak test against `inventory-api`/
  `inventory-worker` (~245k requests, mixed success/error paths across HTTP,
  gRPC, SQL, Redis, Kafka, MongoDB, and Cassandra) - no goroutine or memory
  leak found. See [Stability](README.md#stability) for the exact scope this
  covers (and doesn't).
- Every module also has a baseline-vs-instrumented allocation benchmark
  (`make bench`), which is short-burst, not sustained-load, verification.
- Added `SECURITY.md`, `CHANGELOG.md` (this file), and a README
  [Stability](README.md#stability) section.
- CI green across every module in the workspace at release time; zero known
  open vulnerabilities.
