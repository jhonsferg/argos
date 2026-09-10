# orders-api

A REST API demonstrating **`net/http` + PostgreSQL + Redis** under Argos, with a
cache-aside read path: `GET /orders/{id}` checks Redis first, and only queries
Postgres (populating the cache for next time) on a miss.

## What this demonstrates

| Concern                      | Argos module                      | Where                                                                |
| ---------------------------- | --------------------------------- | -------------------------------------------------------------------- |
| HTTP server tracing/metrics  | `integrations/httpserver/nethttp` | `main.go` - `argosnethttp.Middleware()` wraps the `http.ServeMux`    |
| SQL driver tracing/metrics   | `integrations/sql`                | `main.go` - `argossql.Open("pgx", dsn, ...)`                         |
| Redis client tracing/metrics | `integrations/redis`              | `main.go` - `argosredis.NewHook(...)` added via `client.AddHook`     |
| Init/Shutdown/Logger         | `core`                            | `main.go` - `argos.Init` / `provider.Shutdown` / `provider.Logger()` |

`store.go` holds the cache-aside logic (`OrderStore.Get`); `handlers.go` holds the
HTTP layer, written against a small `Store` interface so it's testable without a
real database (see `handlers_test.go`).

## Run it

```bash
docker compose up -d      # postgres:16-alpine, redis:7-alpine
go run .                  # listens on :8080
```

```bash
curl http://localhost:8080/healthz

curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_name":"ana","item":"widget","quantity":3}'
# {"id":1,"customer_name":"ana","item":"widget","quantity":3,"created_at":"..."}

curl http://localhost:8080/orders/1
# same response - first call is a Postgres read (cache miss), second is a Redis
# hit; confirm with: docker exec orders-api-redis-1 redis-cli GET order:1
```

```bash
docker compose down -v
```

## Configuration

| Env var                       | Default                                                                                     |
| ----------------------------- | ------------------------------------------------------------------------------------------- |
| `HTTP_ADDR`                   | `:8080`                                                                                     |
| `POSTGRES_DSN`                | `postgres://orders:orders@localhost:5432/orders?sslmode=disable`                            |
| `REDIS_ADDR`                  | `localhost:6379`                                                                            |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` (Argos fails open if unreachable - no collector needed to run this sample) |

To see traces/metrics, run the repo's root `docker/docker-compose.otel.yml`
(`make otel-up` from the repo root) and they'll be picked up automatically.
