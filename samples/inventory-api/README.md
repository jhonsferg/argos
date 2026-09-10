# inventory-api

The richest sample in this repo: a net/http API that reserves stock via a
**local gRPC call**, backed by **PostgreSQL** with a **Redis** cache-aside
read layer, publishing a **Kafka** event on every successful reservation.
One `POST /orders` request produces a single trace spanning
HTTP -> gRPC -> Postgres -> Kafka producer. Its companion sample,
[`inventory-worker`](../inventory-worker), consumes that Kafka event and
continues the same trace into MongoDB and Cassandra writes.

## What this demonstrates

| Concern                                    | Argos module/feature           | Where                                                                         |
| ------------------------------------------ | ------------------------------ | ----------------------------------------------------------------------------- |
| HTTP server tracing/metrics + body capture | `integrations/httpserver/core` | `main.go` - `httpservercore.WithCaptureBodyOnError(4096)`                     |
| gRPC server + client tracing/metrics       | `integrations/grpc`            | `main.go` - server and client interceptors on the same process's loopback RPC |
| SQL tracing/metrics                        | `integrations/sql`             | `store.go`                                                                    |
| Redis cache-aside                          | `integrations/redis`           | `store.go`                                                                    |
| Kafka producer + trace propagation         | `integrations/kafka`           | `events.go`                                                                   |
| One-line business-logic tracing            | `argos.TraceFunc`              | `grpcserver.go` - wraps the actual stock reservation                          |
| Global ambient logging                     | `core/log`                     | `store.go` - no `Logger` instance passed to `NewItemStore`                    |
| Startup/shutdown/signal-handling           | `argos.Run`                    | `main.go`                                                                     |

`POST /orders` calling the local gRPC service (rather than doing the
reservation directly) is deliberate: it's what makes the resulting trace
span two "hops" (HTTP server span -> gRPC client span -> gRPC server span)
even though everything runs in one process here - the same shape a real
HTTP-API-in-front-of-a-gRPC-service architecture produces.

## Run it

```bash
docker compose up -d      # postgres:18-alpine, redis:8-alpine, kafka (apache/kafka:4.0.0, KRaft mode)
go run .                  # HTTP on :8086, gRPC on :9091
```

```bash
curl http://localhost:8086/healthz

curl http://localhost:8086/items/widget
# {"id":"widget","name":"Widget","stock":100}

curl -X POST http://localhost:8086/orders \
  -H "Content-Type: application/json" \
  -d '{"item_id":"widget","quantity":5}'
# {"item_id":"widget","remaining":95}

curl -X POST http://localhost:8086/orders \
  -H "Content-Type: application/json" \
  -d '{"item_id":"widget","quantity":10000}'
# 409 insufficient stock - and, because WithCaptureBodyOnError is on servers
# that return 5xx (not this 409), only an actual 5xx captures the request
# body onto the span; try POSTing to a stopped Postgres to see that path.
```

```bash
docker compose down -v
```

## See the trace

With the repo's observability stack running (`make otel-up` from the repo
root - see [Configuration](../../docs/guides/configuration.md)), open
Grafana at `http://localhost:3000` (admin/admin), Explore -> Tempo, and
search for `service.name = inventory-api`. One `POST /orders` trace shows
the HTTP server span, the gRPC client+server span pair, the Postgres query
span, and the Kafka producer span together.

## Configuration

| Env var                       | Default                                                       |
| ----------------------------- | ------------------------------------------------------------- |
| `HTTP_ADDR`                   | `:8086`                                                       |
| `GRPC_ADDR`                   | `:9091`                                                       |
| `POSTGRES_DSN`                | `postgres://argos:argos@localhost:5432/argos?sslmode=disable` |
| `REDIS_ADDR`                  | `localhost:6379`                                              |
| `KAFKA_BROKER`                | `localhost:9092`                                              |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` (Argos fails open if unreachable)            |

## Regenerating the protobuf code

`inventorypb/*.pb.go` is checked in - running the sample needs no `protoc`.
To regenerate after editing `proto/inventory.proto`, using `buf` (no local
protoc/plugins needed beyond building the two Go plugins once):

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/plugins/protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go
GOOS=linux GOARCH=amd64 go build -o /tmp/plugins/protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc
docker run --rm -v "$(pwd)":/work -v /tmp/plugins:/plugins -w /work \
  --entrypoint sh bufbuild/buf:latest -c 'PATH=/plugins:$PATH buf generate proto'
```
