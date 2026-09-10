# GCP Pub/Sub

`integrations/gcppubsub` wraps `cloud.google.com/go/pubsub/v2`'s
`Publisher`/`Subscriber` as free functions.

```go
import (
	"cloud.google.com/go/pubsub/v2"

	argosgcppubsub "github.com/jhonsferg/argos/integrations/gcppubsub"
)

// producer side
serverID, err := argosgcppubsub.Publish(ctx, publisher, &pubsub.Message{Data: payload})

// consumer side
err := argosgcppubsub.Receive(ctx, subscriber, func(ctx context.Context, m *pubsub.Message) {
	handle(ctx, m)
})
```

Testable locally against the official Pub/Sub emulator - no real GCP project
needed:

```go
client, _ := pubsub.NewClient(ctx, projectID,
	option.WithEndpoint(emulatorHost),
	option.WithoutAuthentication(),
	option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
)
```

## See it running

[`shipments-service`](https://github.com/jhonsferg/argos/tree/main/samples/shipments-service)
runs the emulator via Docker Compose and publishes a `shipment.updated`
event per update.
