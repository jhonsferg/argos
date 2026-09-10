# shipments-service

A REST API demonstrating **echo + MongoDB + GCP Pub/Sub** under Argos: updating
a shipment's status upserts it in Mongo and publishes a `shipment.updated` event
to Pub/Sub (against the official emulator, no GCP account needed).

## What this demonstrates

| Concern                                               | Argos module                   | Where                                                                            |
| ----------------------------------------------------- | ------------------------------ | -------------------------------------------------------------------------------- |
| HTTP server tracing/metrics                           | `integrations/httpserver/echo` | `handlers.go` - `argosecho.Middleware()`                                         |
| MongoDB command tracing/metrics                       | `integrations/mongo`           | `main.go` - `argosmongo.NewMonitor()` set via `options.Client().SetMonitor(...)` |
| Pub/Sub publisher tracing/metrics + trace propagation | `integrations/gcppubsub`       | `events.go` - `argosgcppubsub.Publish(...)`                                      |
| Init/Shutdown/Logger                                  | `core`                         | `main.go`                                                                        |

A shipment is keyed by `order_id` - "create" and "update" are the same
`Upsert` operation, matching how a shipment's status actually changes over its
lifecycle (`store.go`).

## Run it

```bash
docker compose up -d      # mongo:7, GCP Pub/Sub emulator
go run .                  # listens on :8083 - creates the shipment.updated
                           # topic on startup if it doesn't already exist
```

```bash
curl http://localhost:8083/healthz

curl -X POST http://localhost:8083/shipments \
  -H "Content-Type: application/json" \
  -d '{"order_id":"order-1","status":"in_transit"}'
# {"order_id":"order-1","status":"in_transit","updated_at":"..."}

curl http://localhost:8083/shipments/order-1
```

To confirm the event was published, create a subscription and pull from it
(the emulator has no web UI, but its REST API works with `curl` from the host -
port 8085 is mapped):

```bash
curl -X PUT "http://localhost:8085/v1/projects/shipments-local/subscriptions/verify" \
  -d '{"topic":"projects/shipments-local/topics/shipment.updated"}'
```

then send another update and pull the subscription (any Pub/Sub client
library works against the emulator - see `PUBSUB_EMULATOR_HOST` below).

```bash
docker compose down -v
```

## Configuration

| Env var                       | Default                                            |
| ----------------------------- | -------------------------------------------------- |
| `HTTP_ADDR`                   | `:8083`                                            |
| `MONGO_URI`                   | `mongodb://localhost:27017`                        |
| `PUBSUB_PROJECT_ID`           | `shipments-local`                                  |
| `PUBSUB_EMULATOR_HOST`        | `localhost:8085`                                   |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` (Argos fails open if unreachable) |

To see traces/metrics, run the repo's root `docker/docker-compose.otel.yml`
(`make otel-up` from the repo root).
