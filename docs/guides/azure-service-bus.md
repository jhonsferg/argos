# Azure Service Bus

`integrations/azuresb` wraps `azservicebus.Sender`/`ReceivedMessage` as free
functions, re-exporting the shared `messaging/core` `Option`/`WithLogger`.

```go
import argosazuresb "github.com/jhonsferg/argos/integrations/azuresb"

err := argosazuresb.SendMessage(ctx, sender, msg)

err := argosazuresb.Process(ctx, receivedMsg, func(ctx context.Context, m *azservicebus.ReceivedMessage) error {
	return handle(ctx, m)
})
```

`Process` recovers a panic inside the handler (via `core/middleware`),
recording it on the span and returning it as an error rather than crashing
the receive loop.

!!! note "No local emulator"

    Azure Service Bus has no `testcontainers-go` module or comparably simple
    local emulator (it needs a companion SQL Server/Azurite container and
    manual queue/topic configuration), so this integration's context
    propagation is covered by its unit tests rather than a real-broker
    integration test or a runnable sample - unlike every other messaging
    system in this library.
