# Cassandra

`integrations/cassandra` instruments `github.com/gocql/gocql` via its
`QueryObserver`/`BatchObserver` interfaces.

```go
import (
	"github.com/gocql/gocql"

	argoscassandra "github.com/jhonsferg/argos/integrations/cassandra"
)

obs := argoscassandra.New()
cluster := gocql.NewCluster("localhost")
cluster.Keyspace = "myapp"
cluster.QueryObserver = obs
cluster.BatchObserver = obs
session, err := cluster.CreateSession()
```

`cluster.Keyspace` must already exist before `CreateSession` - create it via
a throwaway session with no keyspace set first, same as any `gocql` usage.

## Options

- `WithQueryText(bool)` - records the CQL statement. Off by default.
- `WithMaskedQueryText(bool)` - records the CQL statement with string and
  numeric literals replaced by `?`. Masked wins if both are enabled.
- `WithLogger(logging.Logger)`.
- `WithYAMLConfig(cfg core.Config)` - `query_text`/`query_text_masked`
  under `integrations.cassandra`.

## See it running

[`reports-service`](https://github.com/jhonsferg/argos/tree/main/samples/reports-service)
records pulled-report metadata in Cassandra.
