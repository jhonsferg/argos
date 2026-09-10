# users-service

Two binaries demonstrating **gorilla/mux + gRPC** under Argos: `gateway` is an
HTTP↔gRPC translator, `backend` is the actual gRPC service (an in-memory user
store - no database needed, since this sample is about the router+gRPC
wiring, not a persistence integration those other samples already cover).
There's no external infrastructure to run - the simplest sample to try first.

## What this demonstrates

| Concern                                         | Argos module                         | Where                                                                                           |
| ----------------------------------------------- | ------------------------------------ | ----------------------------------------------------------------------------------------------- |
| HTTP server tracing/metrics                     | `integrations/httpserver/gorillamux` | `cmd/gateway/handlers.go` - `argosgorillamux.Middleware()`                                      |
| gRPC server tracing/metrics                     | `integrations/grpc`                  | `cmd/backend/main.go` - `argosgrpc.UnaryServerInterceptor()`                                    |
| gRPC client tracing/metrics + trace propagation | `integrations/grpc`                  | `cmd/gateway/main.go` - `argosgrpc.UnaryClientInterceptor()` on the gateway's `grpc.ClientConn` |
| Init/Shutdown/Logger                            | `core`                               | both `main.go` files                                                                            |

The trace started by `argosgorillamux.Middleware()` on an incoming HTTP
request continues automatically into the outgoing gRPC call made by the
gateway - one trace spans both processes.

`cmd/gateway/handlers.go` depends on the generated `userspb.UsersClient`
interface directly (no hand-written interface needed) so tests
(`handlers_test.go`) can supply a fake without a real gRPC backend.

## Regenerating the protobuf code

`userspb/*.pb.go` is checked in - running the sample needs no `protoc`. To
regenerate after editing `proto/users.proto`:

```bash
protoc --go_out=userspb --go_opt=paths=source_relative \
  --go-grpc_out=userspb --go-grpc_opt=paths=source_relative \
  proto/users.proto
```

## Run it

```bash
go run ./cmd/backend    # gRPC server on :9090
go run ./cmd/gateway    # HTTP gateway on :8085 (in a second terminal)
```

```bash
curl http://localhost:8085/healthz

curl -X POST http://localhost:8085/users \
  -H "Content-Type: application/json" \
  -d '{"name":"ana","email":"ana@example.com"}'
# {"id":"<uuid>","name":"ana","email":"ana@example.com"}

curl http://localhost:8085/users/<uuid>

curl http://localhost:8085/users/does-not-exist
# 404 - the gateway translates the backend's gRPC NotFound into an HTTP status
```

## Configuration

| Env var                       | Default                                            | Binary  |
| ----------------------------- | -------------------------------------------------- | ------- |
| `GRPC_ADDR`                   | `:9090`                                            | backend |
| `HTTP_ADDR`                   | `:8085`                                            | gateway |
| `BACKEND_ADDR`                | `localhost:9090`                                   | gateway |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` (Argos fails open if unreachable) | both    |

To see traces/metrics, run the repo's root `docker/docker-compose.otel.yml`
(`make otel-up` from the repo root).
