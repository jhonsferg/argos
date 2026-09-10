# Changelog

All notable changes to this repository are documented here, across every module,
in one chronological log - see [Repository layout](README.md#repository-layout)
for why this isn't split per module. Each module is versioned and tagged
independently (`<module-path>/vX.Y.Z`); an entry below names the specific
module(s) it applies to.

## [Unreleased]

Changes merged to `main` since the `v0.1.0` release wave, not yet tagged as a new
version of any module (test-only and CI-only changes don't require a release and
aren't listed as pending one below).

- **All of `capture`, `integrations/{sql,gorm,redis,mongo,cassandra}`,
  `integrations/httpclient`, `integrations/httpserver/core`, `integrations/grpc`**:
  configurable capture rules - `WithMaskedQueryText` (redacted query text) on the
  five database integrations, and full request/response body + header capture
  (`WithCapture`, plus the existing `WithCaptureBodyOnError` sugar) on the HTTP
  client, HTTP server core, and gRPC client. All backed by the new `capture`
  package (`capture.MaskSQL`, `capture.ApplyHeaderRule`) and configurable via both
  functional options and YAML.
- **`capture`**: `capture.MaskSQL` was fuzz-tested (see
  `capture/fuzz_test.go`), which found and fixed a real bug - an apostrophe
  inside a SQL comment or a double-quoted identifier could desynchronize the
  masking regex and let a real literal value (e.g. an email address) leak
  unmasked into span attributes. Numeric literal masking was also extended to
  hexadecimal and scientific notation. See
  [docs/guides/sql.md](docs/guides/sql.md) for the verified guarantees and known
  limitations this uncovered.
- Real per-module test coverage was measured (not just pass/fail) and the
  highest-risk gaps closed: `integrations/sql`'s entire legacy
  `database/sql/driver` fallback path (the reason the package exists, per its own
  doc comment) had zero coverage; `WithLogger` was untested in every single
  DB/messaging integration; `integrations/httpserver/core`'s streaming/hijacking
  compatibility layer (`Flush`/`Unwrap`) was untested.
- Release pipeline (`release.yml`) now explicitly rejects tags under `samples/**`
  or `examples/**` - these are reference code, never meant to be published as
  versioned modules.
- `Benchmark` workflow switched from running on every PR to manual dispatch only,
  to stop it bottlenecking normal CI - see the open follow-up to re-add a
  non-blocking scheduled run compared against a persisted baseline.
- CI now uploads per-module coverage as a non-blocking artifact (`ci.yml` for
  every module's unit tests, `integration-tests.yml` for the 10 modules whose
  real coverage only shows up under `-tags=integration`) - see the
  [Stability](README.md#stability) section for why `-coverpkg=./...` was
  required to avoid a misleading number.
- Added `SECURITY.md`, `CHANGELOG.md` (this file), and a README
  [Stability](README.md#stability) section.
- Ran a 1-hour sustained-load soak test against `inventory-api`/
  `inventory-worker` (~245k requests, mixed success/error paths across HTTP,
  gRPC, SQL, Redis, Kafka, MongoDB, and Cassandra) - no goroutine or memory
  leak found. See [Stability](README.md#stability) for the exact scope this
  covers (and doesn't).
- Fixed every one of the 31 (of 33) modules that imported the old
  `github.com/jhonsferg/argos/core` module without declaring it as a direct
  `require` in their own `go.mod` - invisible under `go.work`, but meant
  `GOWORK=off go build` (an isolated build of just that module, the way any
  tool other than this exact workspace checkout would build it) failed.
- **Breaking (unreleased): the root module moved from
  `github.com/jhonsferg/argos/core` to `github.com/jhonsferg/argos`.**
  `package argos` was always the right name for this code - only its import
  path was wrong, nested under a redundant `core` subdirectory instead of
  living at the repository root the way `go get github.com/jhonsferg/argos`
  implies. No compatibility shim for the old path - every module in this
  repo is still `v0.x`, see [Stability](README.md#stability). Every
  consuming module's `go.mod` now requires `github.com/jhonsferg/argos`
  instead of `.../argos/core`. **This is not fully released yet**: no tag
  has been cut for the new root module path, so `go.work` carries a
  temporary `replace github.com/jhonsferg/argos v0.1.0 => ./` until a
  coordinated tagging pass (new tag for the root module, plus a patch tag
  for every affected integration) ships - see the open follow-up.

## [integrations/gcppubsub v0.1.1] - 2026-09-09

- Bump `google.golang.org/grpc` to `v1.83.2` and declare the `core` and
  `integrations/messaging/core` requires this module's `go.mod` was missing -
  found via `GOWORK=off go build` verification (compiling with the workspace
  disabled, the way an external consumer actually resolves dependencies), not by
  any check that runs with `go.work` active.

## [v0.1.0] - 2026-09-09

Initial release of every module in this repository:

- **`core`**: `Init`/`Shutdown`/`Run`, functional-options or YAML configuration,
  the global ambient logger, the `Trace`/`TraceFunc` span-wrap helper, panic
  recovery middleware, and `core/argostest` test fixtures. Zero dependencies on
  any vendor SDK.
- **Integrations**: tracing, metrics, and correlated logging for SQL (any
  `database/sql` driver), GORM, Redis, MongoDB, Cassandra, Kafka, RabbitMQ, gRPC,
  HTTP client, six HTTP server routers (net/http, chi, gin, echo, fiber,
  gorilla/mux), SFTP, SMTP, GCP Pub/Sub, and Azure Service Bus - one Go module
  per integration, each with its own `go.mod` so consumers only pull in the
  vendor SDK they actually use.
- **`cmd/doctor`**: a CLI that scans a Go project's own imports and suggests any
  Argos integration matching a vendor SDK it already uses but hasn't wired up.
- **Samples**: eight runnable microservices verified end to end against real
  Docker infrastructure, covering every integration above in realistic
  combinations.
- CI green across all 34 modules in the workspace at release time; zero known
  open vulnerabilities.
