<div align="center">

# Argos

**Zero-allocation-focused declarative OpenTelemetry instrumentation for Go microservices.**

[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=for-the-badge&logo=go)](https://pkg.go.dev/github.com/jhonsferg/argos)
[![CI](https://img.shields.io/github/actions/workflow/status/jhonsferg/argos/ci.yml?style=for-the-badge&logo=github&label=CI)](https://github.com/jhonsferg/argos/actions/workflows/ci.yml)
[![Lint](https://img.shields.io/github/actions/workflow/status/jhonsferg/argos/lint.yml?style=for-the-badge&logo=github&label=Lint)](https://github.com/jhonsferg/argos/actions/workflows/lint.yml)
[![Security Scan](https://img.shields.io/github/actions/workflow/status/jhonsferg/argos/security-scan.yml?style=for-the-badge&logo=github&label=Security%20Scan)](https://github.com/jhonsferg/argos/actions/workflows/security-scan.yml)
[![CodeQL](https://img.shields.io/github/actions/workflow/status/jhonsferg/argos/codeql.yml?style=for-the-badge&logo=github&label=CodeQL)](https://github.com/jhonsferg/argos/actions/workflows/codeql.yml)
[![pkg.go.dev](https://img.shields.io/badge/pkg.go.dev-reference-007D9C?style=for-the-badge&logo=go)](https://pkg.go.dev/github.com/jhonsferg/argos)
[![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-brightgreen?style=for-the-badge)](https://goreportcard.com/report/github.com/jhonsferg/argos)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)](LICENSE)

<img src="assets/argos-pet.png" alt="Argos mascot" width="220">

---

**[Documentation](https://jhonsferg.github.io/argos) · [Quick Start](#quick-start) · [Integrations](#integrations) · [Samples](#samples) · [Tools](#tools) · [Stability](#stability)**

</div>

## Overview

Go has no equivalent to Java's `-javaagent` bytecode instrumentation, and the only
real "zero-code" alternative (eBPF) needs elevated privileges, a recent Linux kernel,
and covers a limited set of libraries. Argos takes the low-code path instead: one
`argos.Init()` call at startup, then an explicit, drop-in wrapper per client library
you actually use - no monkey-patching, no reflection on the hot path.

The root module has **zero dependencies on any vendor SDK**. Every integration
(Kafka, Redis, gRPC, GORM, ...) lives in its own Go module with its own `go.mod`, so
importing one never pulls in another's transitive dependencies.

```bash
go get github.com/jhonsferg/argos
```

Requires Go 1.26 or later - earlier versions are not supported.

---

## Quick Start

```go
package main

import (
	"context"
	"log"
	"net/http"

	argos "github.com/jhonsferg/argos"
	argosnethttp "github.com/jhonsferg/argos/integrations/httpserver/nethttp"
)

func main() {
	err := argos.Run(context.Background(), []argos.Option{
		argos.WithServiceName("my-service"),
	}, func(ctx context.Context, provider *argos.Provider) error {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
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

`argos.Run` handles `Init`, SIGINT/SIGTERM-triggered shutdown, and `Shutdown` in one
call - see the [Quick Start guide](https://jhonsferg.github.io/argos/quickstart/) for
the lower-level `Init`/`Shutdown` pair when you need more control.

By default, traces/metrics export via OTLP to `localhost:4317` and **fail open** -
requests keep working even with no collector running. See the
[Quick Start guide](https://jhonsferg.github.io/argos/quickstart/) for the full
walkthrough, including how to point at a real collector, and the
[Configuration guide](https://jhonsferg.github.io/argos/guides/configuration/)
for loading `Init` from a YAML file instead of (or alongside) functional options.

---

## Integrations

| Module            | Wraps                                        | Import path                        |
| ----------------- | -------------------------------------------- | ---------------------------------- |
| HTTP servers      | net/http, chi, gin, echo, fiber, gorilla/mux | `integrations/httpserver/<router>` |
| HTTP client       | any `http.RoundTripper`                      | `integrations/httpclient`          |
| SQL               | any `database/sql` driver                    | `integrations/sql`                 |
| GORM              | `gorm.io/gorm`                               | `integrations/gorm`                |
| Redis             | `github.com/redis/go-redis/v9`               | `integrations/redis`               |
| MongoDB           | `go.mongodb.org/mongo-driver/v2`             | `integrations/mongo`               |
| Cassandra         | `github.com/gocql/gocql`                     | `integrations/cassandra`           |
| Kafka             | `github.com/IBM/sarama`                      | `integrations/kafka`               |
| RabbitMQ          | `github.com/rabbitmq/amqp091-go`             | `integrations/rabbitmq`            |
| Azure Service Bus | `azservicebus`                               | `integrations/azuresb`             |
| GCP Pub/Sub       | `cloud.google.com/go/pubsub/v2`              | `integrations/gcppubsub`           |
| gRPC              | `google.golang.org/grpc`                     | `integrations/grpc`                |
| SFTP              | `github.com/pkg/sftp`                        | `integrations/sftp`                |
| SMTP              | `net/smtp`                                   | `integrations/smtp`                |

Every module ships real tests, a comparative baseline-vs-instrumented benchmark, and
passes `golangci-lint`/`gosec`/`govulncheck` clean. See the
[guides](https://jhonsferg.github.io/argos/guides/http-servers/) for wiring snippets,
or [`integrations/contrib`](integrations/contrib) for a template + decision tree if
your library isn't covered yet.

---

## Samples

Eight runnable microservices, verified end to end against real Docker infrastructure.
Six mix a different router + one data/messaging stack each; the last two -
`inventory-api` + `inventory-worker` - are a cooperating pair combining several
integrations into one request that produces a single trace spanning both processes:

| Sample                                                 | Stack                                        |
| ------------------------------------------------------ | -------------------------------------------- |
| [`orders-api`](samples/orders-api)                     | net/http + PostgreSQL + Redis (cache-aside)  |
| [`catalog-service`](samples/catalog-service)           | gin + GORM/PostgreSQL + Kafka                |
| [`notifications-worker`](samples/notifications-worker) | chi + RabbitMQ + SMTP                        |
| [`shipments-service`](samples/shipments-service)       | echo + MongoDB + GCP Pub/Sub                 |
| [`reports-service`](samples/reports-service)           | fiber + Cassandra + SFTP                     |
| [`users-service`](samples/users-service)               | gorilla/mux + gRPC (no external infra)       |
| [`inventory-api`](samples/inventory-api)               | net/http + gRPC + PostgreSQL + Redis + Kafka |
| [`inventory-worker`](samples/inventory-worker)         | Kafka consumer + MongoDB + Cassandra         |

Each has its own `docker-compose.yml` and README with exact run/curl instructions.
Every backend system these samples use can also be brought up at once via
`make systems-up`, independent of any one sample.

---

## Tools

`cmd/doctor` scans a Go project's own imports and suggests any Argos integration
matching a vendor SDK it already uses but hasn't wired up yet:

```bash
go run github.com/jhonsferg/argos/cmd/doctor [-strict] [path]
```

`-strict` exits with status 1 if any integration is suggested, for use as a CI check.
`-init` instead scaffolds a starter `argos.config.yaml` (plus a ready-to-paste
`main.go` snippet) for a new service - see the
[Doctor CLI guide](https://jhonsferg.github.io/argos/guides/doctor-cli/).

---

## Repository layout

Go workspace (`go.work`) multi-module repo:

```
argos/                Init/Shutdown/Run, Config (functional options or YAML),
│                     Trace/TraceFunc, Resource, Propagation, Logger + global
│                     ambient logging, argostest - the repo root is the main
│                     module (`go get github.com/jhonsferg/argos`), zero
│                     vendor SDK dependencies
├── integrations/     One Go module per vendor SDK (see the table above)
├── cmd/doctor/       The CLI described above
├── samples/          Eight runnable microservices (see above)
├── examples/         Minimal single-file smoke tests
├── docs/             This documentation site (mkdocs-material)
└── docker/           Grafana LGTM + OTel Collector stack, plus every backend
                      system argos instruments - see docs/guides/configuration.md
```

See [Architecture](https://jhonsferg.github.io/argos/architecture/) for the full
design principles: the zero-allocation discipline every module is held to, and the
three shapes used to wrap a third-party SDK depending on what it exposes.

## Stability

Every module in this repository is still `v0.x` - the public API of any module
can still change between minor versions, though every existing release stays
resolvable and unaffected (Go module versioning, not this repo's history,
governs that). See [CHANGELOG.md](CHANGELOG.md) for what's shipped so far and
[SECURITY.md](SECURITY.md) for how vulnerabilities are handled given that.

Specifically on the opt-in capture/masking features
(`capture.MaskSQL`, `WithMaskedQueryText`, `WithCapture`,
`WithCaptureBodyOnError`): `capture.MaskSQL` has been verified by sustained fuzz
testing (`core/capture/fuzz_test.go`), which found and fixed a real bug before
this note was written - see [docs/guides/sql.md](docs/guides/sql.md#options) for
the specific guarantees that testing verified and the known limitations it
found, instead of a generic "best effort" disclaimer. The other capture
features (header/body capture on the HTTP client, HTTP server, and gRPC
integrations) are covered by unit tests but have not had the same fuzzing pass.

On sustained-load behavior: a 1-hour soak test against the
[`inventory-api`](samples/inventory-api)/[`inventory-worker`](samples/inventory-worker)
pair (HTTP server, gRPC client+server, SQL, Redis, Kafka producer+consumer,
MongoDB, and Cassandra in one request path), under continuous mixed
success/error traffic (~245k requests across 4 concurrent scenarios), found no
goroutine or memory growth - both settled to a flat plateau after an initial
few minutes of warm-up and stayed there for the rest of the run. That's real
evidence for the integrations it exercises, not a blanket guarantee for every
integration in the table above (RabbitMQ, GCP Pub/Sub, Azure Service Bus, SFTP,
and SMTP haven't had a dedicated soak test) or for runs longer than an hour.
Every module also has a baseline-vs-instrumented allocation benchmark
(`make bench`), which is short-burst, not sustained-load, verification.

## Development

```
make build     # go build across every module in go.work
make test      # go test across every module
make bench     # go test -bench=. -benchmem across every module
make lint      # golangci-lint (containerized)
make fmt       # golangci-lint fmt + Prettier + buf format, all containerized
make sec-scan  # gosec + govulncheck + trivy (containerized)
make otel-up / make otel-down       # Grafana LGTM + OTel Collector stack
make systems-up / make systems-down # every backend system argos instruments
```

## License

[MIT](LICENSE)
