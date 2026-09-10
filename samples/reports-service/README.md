# reports-service

A REST API demonstrating **fiber + Cassandra + SFTP** under Argos: pulling a
report fetches it from an SFTP server and records its metadata in Cassandra.

## What this demonstrates

| Concern                                      | Argos module                    | Where                                                                             |
| -------------------------------------------- | ------------------------------- | --------------------------------------------------------------------------------- |
| HTTP server tracing/metrics (fasthttp-based) | `integrations/httpserver/fiber` | `handlers.go` - `argosfiber.Middleware()`                                         |
| Cassandra query/batch tracing/metrics        | `integrations/cassandra`        | `main.go` - `argoscassandra.New()` set as `cluster.QueryObserver`/`BatchObserver` |
| SFTP client tracing/metrics                  | `integrations/sftp`             | `reports.go` - `argossftp.Wrap(...)`                                              |
| Init/Shutdown/Logger                         | `core`                          | `main.go`                                                                         |

Fiber runs on fasthttp, not `net/http` - handlers must read the span-carrying
context via `c.UserContext()` (set by `argosfiber.Middleware`), not the raw
`c.Context()`; see the comment in `handlers.go`.

The sample only measures a pulled file's size rather than parsing its content
(`reports.go`) - that's business logic outside what this sample demonstrates,
which is the SFTP wrapper itself.

## Run it

```bash
docker compose up -d      # cassandra:5, atmoz/sftp (seeded with testdata/sample-report.csv)
go run .                  # listens on :8084
```

```bash
curl http://localhost:8084/healthz

curl -X POST http://localhost:8084/reports/pull \
  -H "Content-Type: application/json" \
  -d '{"path":"/upload/sample-report.csv"}'
# {"filename":"sample-report.csv","size_bytes":72,"pulled_at":"..."}

curl http://localhost:8084/reports/sample-report.csv
```

```bash
docker compose down -v
```

## Configuration

| Env var                       | Default                                            |
| ----------------------------- | -------------------------------------------------- |
| `HTTP_ADDR`                   | `:8084`                                            |
| `CASSANDRA_HOST`              | `localhost`                                        |
| `CASSANDRA_KEYSPACE`          | `reports`                                          |
| `SFTP_ADDR`                   | `localhost:2222`                                   |
| `SFTP_USER` / `SFTP_PASSWORD` | `reports` / `reports`                              |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` (Argos fails open if unreachable) |

To see traces/metrics, run the repo's root `docker/docker-compose.otel.yml`
(`make otel-up` from the repo root).
