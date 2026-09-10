# MongoDB

`integrations/mongo` instruments the official `go.mongodb.org/mongo-driver/v2`
via its `event.CommandMonitor` - a documented extension point, no client
wrapping needed.

```go
import (
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	argosmongo "github.com/jhonsferg/argos/integrations/mongo"
)

client, err := mongo.Connect(options.Client().
	ApplyURI("mongodb://localhost:27017").
	SetMonitor(argosmongo.NewMonitor()))
```

## Options

- `WithQueryText(bool)` - records the command document. Off by default.
- `WithMaskedQueryText(bool)` - records the command with top-level field
  values redacted, keys kept (e.g. `{insert: ?, documents: ?}`). Masked
  wins if both are enabled.
- `WithLogger(logging.Logger)`.
- `WithYAMLConfig(cfg core.Config)` - `query_text`/`query_text_masked`
  under `integrations.mongo`.

## See it running

[`shipments-service`](https://github.com/jhonsferg/argos/tree/main/samples/shipments-service)
upserts shipment status in Mongo and publishes a GCP Pub/Sub event per
update.
