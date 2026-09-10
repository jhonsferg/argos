# Redis

`integrations/redis` instruments `github.com/redis/go-redis/v9` via its
`redis.Hook` interface - no wrapping of the client itself needed.

```go
import (
	"github.com/redis/go-redis/v9"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	argosredis "github.com/jhonsferg/argos/integrations/redis"
)

client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
client.AddHook(argosredis.NewHook(argosredis.WithSystem(semconv.DBSystemRedis)))
```

## Options

- `WithSystem(attribute.KeyValue)` - sets `db.system`.
- `WithQueryText(bool)` - records the command as `db.query.text`. Off by
  default (command arguments can contain values you don't want on a span).
- `WithMaskedQueryText(bool)` - records the command name with every
  argument replaced by `?` (e.g. `SET ? ?` instead of
  `SET session:abc123 secret-token`). Masked wins if both are enabled.
- `WithLogger(logging.Logger)`.
- `WithYAMLConfig(cfg core.Config)` - `query_text`/`query_text_masked`
  under `integrations.redis`.

## See it running

[`orders-api`](https://github.com/jhonsferg/argos/tree/main/samples/orders-api)
uses it as a cache-aside layer in front of Postgres reads.
