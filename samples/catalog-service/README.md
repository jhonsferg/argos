# catalog-service

A REST API demonstrating **gin + PostgreSQL/GORM + Kafka** under Argos: creating
an item persists it via GORM and publishes a `catalog.item.created` event to
Kafka in the same request.

## What this demonstrates

| Concern                                            | Argos module                  | Where                                                                                                         |
| -------------------------------------------------- | ----------------------------- | ------------------------------------------------------------------------------------------------------------- |
| HTTP server tracing/metrics                        | `integrations/httpserver/gin` | `handlers.go` - `argosgin.Middleware()`                                                                       |
| ORM-level tracing/metrics                          | `integrations/gorm`           | `main.go` - `argosgorm.New(...)` registered via `db.Use(...)`                                                 |
| Underlying driver tracing/metrics                  | `integrations/sql`            | `main.go` - GORM opens on top of `argossql.Open("pgx", ...)`, so both layers produce spans for the same query |
| Kafka producer tracing/metrics + trace propagation | `integrations/kafka`          | `events.go` - `argoskafka.Send(...)` injects the active trace context into the message headers                |
| Init/Shutdown/Logger                               | `core`                        | `main.go`                                                                                                     |

A publish failure never rolls back the database write - the row is the source
of truth, the event is a best-effort side effect (see
`handleCreateItem` in `handlers.go`, covered by
`TestHandleCreateItem_PublishFailureStillCreates`).

## Run it

```bash
docker compose up -d      # postgres:16-alpine, redpanda (single-node, Kafka API-compatible)
go run .                  # listens on :8081
```

```bash
curl http://localhost:8081/healthz

curl -X POST http://localhost:8081/items \
  -H "Content-Type: application/json" \
  -d '{"name":"widget","price_cents":1999}'
# {"id":1,"name":"widget","price_cents":1999}

curl http://localhost:8081/items/1

# confirm the event was published, with trace context attached:
docker exec catalog-service-redpanda-1 rpk topic consume catalog.item.created --num 1
```

```bash
docker compose down -v
```

## Configuration

| Env var                       | Default                                                             |
| ----------------------------- | ------------------------------------------------------------------- |
| `HTTP_ADDR`                   | `:8081`                                                             |
| `POSTGRES_DSN`                | `postgres://catalog:catalog@localhost:5432/catalog?sslmode=disable` |
| `KAFKA_BROKER`                | `localhost:9092`                                                    |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` (Argos fails open if unreachable)                  |

To see traces/metrics, run the repo's root `docker/docker-compose.otel.yml`
(`make otel-up` from the repo root).
