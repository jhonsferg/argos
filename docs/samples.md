# Samples

Eight runnable microservices. Six mix a different combination of HTTP
router and one data/messaging technology each; the last two -
[`inventory-api`](https://github.com/jhonsferg/argos/tree/main/samples/inventory-api)
and
[`inventory-worker`](https://github.com/jhonsferg/argos/tree/main/samples/inventory-worker) -
are a cooperating pair that combine several integrations into one realistic
system, producing a single trace that spans both processes. Every sample
has its own `docker-compose.yml` for its backing services (where it needs
any), a `README.md` with exact run/curl instructions, and was verified end
to end against real infrastructure - not just written and assumed to work.

Together they cover all six HTTP router adapters and every messaging/data
integration except Azure Service Bus (no local emulator exists - see the
[Azure Service Bus guide](guides/azure-service-bus.md)). Every backend
system these samples depend on can also be brought up at once, independent
of any sample, via `make systems-up` (see
[Configuration](guides/configuration.md)).

| Sample                                                                                              | Stack                                        | Demonstrates                                                                        |
| --------------------------------------------------------------------------------------------------- | -------------------------------------------- | ----------------------------------------------------------------------------------- |
| [`orders-api`](https://github.com/jhonsferg/argos/tree/main/samples/orders-api)                     | net/http + PostgreSQL + Redis                | Cache-aside reads: Redis checked first, Postgres on a miss                          |
| [`catalog-service`](https://github.com/jhonsferg/argos/tree/main/samples/catalog-service)           | gin + GORM/PostgreSQL + Kafka                | Publishing a domain event on write, with propagated trace context                   |
| [`notifications-worker`](https://github.com/jhonsferg/argos/tree/main/samples/notifications-worker) | chi + RabbitMQ + SMTP                        | A background worker: consume a queue, send an email per message                     |
| [`shipments-service`](https://github.com/jhonsferg/argos/tree/main/samples/shipments-service)       | echo + MongoDB + GCP Pub/Sub                 | Upsert-as-update semantics, publishing to the real Pub/Sub emulator                 |
| [`reports-service`](https://github.com/jhonsferg/argos/tree/main/samples/reports-service)           | fiber + Cassandra + SFTP                     | Pulling a file from a real SFTP server, fasthttp's different context model          |
| [`users-service`](https://github.com/jhonsferg/argos/tree/main/samples/users-service)               | gorilla/mux + gRPC                           | An HTTP↔gRPC gateway; zero external infrastructure needed                           |
| [`inventory-api`](https://github.com/jhonsferg/argos/tree/main/samples/inventory-api)               | net/http + gRPC + PostgreSQL + Redis + Kafka | One request tracing HTTP -> gRPC -> `argos.TraceFunc` -> Postgres -> Redis -> Kafka |
| [`inventory-worker`](https://github.com/jhonsferg/argos/tree/main/samples/inventory-worker)         | Kafka consumer + MongoDB + Cassandra         | Continuing inventory-api's trace into a second process via propagated context       |

Each sample's `go.mod` joins the repo's `go.work`, so they build and test
alongside every integration module - they're held to the same bar (real
tests, `golangci-lint`/`gosec`/`govulncheck` clean), not treated as
throwaway demo code.
