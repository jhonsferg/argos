# inventory-worker

The consumer half of a two-service story with
[`inventory-api`](../inventory-api): this worker consumes the
`inventory.stock.changed` Kafka topic inventory-api publishes to, recording
each event in **MongoDB** (an audit trail) and **Cassandra** (an
append-only analytics log). Because the trace context inventory-api
propagated in the message headers is extracted here, this consumer's span -
and the Mongo/Cassandra spans below it - continue the _same trace_ as the
HTTP request that produced the event, in a completely separate process.

## What this demonstrates

| Concern                           | Argos module             | Where                                                    |
| --------------------------------- | ------------------------ | -------------------------------------------------------- |
| Kafka consumer + trace extraction | `integrations/kafka`     | `consumer.go` - `argoskafka.Consume(...)`                |
| MongoDB tracing/metrics           | `integrations/mongo`     | `main.go`/`store.go`                                     |
| Cassandra tracing/metrics         | `integrations/cassandra` | `main.go`/`store.go`                                     |
| Global ambient logging            | `core/log`               | `store.go` - no `Logger` instance passed to either store |
| Startup/shutdown/signal-handling  | `argos.Run`              | `main.go`                                                |

`EventProcessor` depends on `AuditRecorder`/`AnalyticsAppender` interfaces,
not the concrete Mongo/Cassandra stores, so `consumer_test.go` exercises the
processing logic (including "audit fails, don't append analytics" ordering)
with no real infrastructure.

## Run it

This sample only has something to consume once **inventory-api is already
running** (it owns the shared Kafka broker both services connect to):

```bash
cd ../inventory-api && docker compose up -d && go run .   # starts postgres, redis, kafka + inventory-api itself
```

Then, in this directory:

```bash
docker compose up -d      # mongo:8, cassandra:5 (this worker's own backing stores)
go run .                  # consumes inventory.stock.changed
```

Drive an order through inventory-api (see its README) and watch this
worker's logs - each `POST /orders` there produces one consumed event here.

```bash
docker compose down -v
```

## See the trace

With the repo's observability stack running (`make otel-up` from the repo
root), the trace started by inventory-api's `POST /orders` continues into
this worker's consumer span, MongoDB span, and Cassandra span - search
Tempo for the trace ID logged by inventory-api's `POST /orders` response,
or browse by `service.name = inventory-worker` in Grafana Explore.

## Configuration

| Env var                       | Default                                            |
| ----------------------------- | -------------------------------------------------- |
| `KAFKA_BROKER`                | `localhost:9092`                                   |
| `KAFKA_GROUP`                 | `inventory-worker`                                 |
| `MONGO_URI`                   | `mongodb://localhost:27017`                        |
| `CASSANDRA_HOST`              | `localhost`                                        |
| `CASSANDRA_KEYSPACE`          | `inventory`                                        |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` (Argos fails open if unreachable) |
