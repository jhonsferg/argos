# notifications-worker

A background worker demonstrating **chi + RabbitMQ + SMTP** under Argos: it
consumes a `notifications` queue and sends one email per message. The chi
router only exposes `/healthz` - the worker's real work happens off the queue,
not over HTTP.

## What this demonstrates

| Concern                                               | Argos module                  | Where                                                                                                     |
| ----------------------------------------------------- | ----------------------------- | --------------------------------------------------------------------------------------------------------- |
| Admin HTTP server tracing/metrics                     | `integrations/httpserver/chi` | `handlers.go` - `argoschi.Middleware()`                                                                   |
| RabbitMQ consumer tracing/metrics + trace propagation | `integrations/rabbitmq`       | `worker.go` - `argosrabbitmq.Consume(...)` links the consumer span to whichever producer sent the message |
| SMTP send tracing/metrics                             | `integrations/smtp`           | `mailer.go` - `argossmtp.SendMail(...)`                                                                   |
| Init/Shutdown/Logger                                  | `core`                        | `main.go`                                                                                                 |

`worker.go` is written against a small `Mailer` interface (`mailer.go`) so the
message-parsing/dispatch logic is testable without a real SMTP server -
see `worker_test.go`.

## Run it

```bash
docker compose up -d      # rabbitmq:3-management-alpine, mailpit
go run .                  # consumes "notifications"; admin API on :8082
```

Publish a test message (RabbitMQ's management API, no extra tooling needed):

```bash
docker exec notifications-worker-rabbitmq-1 rabbitmqadmin publish \
  routing_key=notifications \
  payload='{"to":"someone@example.com","subject":"Order shipped","body":"Your order is on its way!"}'
```

Check it arrived - Mailpit's web UI is at http://localhost:8025, or via its API:

```bash
curl http://localhost:8025/api/v1/messages
```

```bash
docker compose down -v
```

## Configuration

| Env var                       | Default                                            |
| ----------------------------- | -------------------------------------------------- |
| `HTTP_ADDR`                   | `:8082`                                            |
| `RABBITMQ_URL`                | `amqp://guest:guest@localhost:5672/`               |
| `SMTP_ADDR`                   | `localhost:1025`                                   |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` (Argos fails open if unreachable) |

To see traces/metrics, run the repo's root `docker/docker-compose.otel.yml`
(`make otel-up` from the repo root).
