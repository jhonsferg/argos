# Kafka

`integrations/kafka` wraps `github.com/IBM/sarama`'s producer/consumer as
free functions - `sarama.SyncProducer` has no interface to hook and no
`context.Context` parameter, so this is the same free-function shape used
for every "concrete SDK type" integration in this library.

```go
import (
	"github.com/IBM/sarama"

	argoskafka "github.com/jhonsferg/argos/integrations/kafka"
)

// producer side
partition, offset, err := argoskafka.Send(ctx, producer, &sarama.ProducerMessage{
	Topic: "orders",
	Value: sarama.StringEncoder("hello"),
})

// consumer side - context propagates from whichever producer sent msg
err := argoskafka.Consume(ctx, msg, func(ctx context.Context, msg *sarama.ConsumerMessage) error {
	return handle(ctx, msg)
})
```

Trace context travels in the message headers, which requires
`cfg.Version = sarama.V2_8_0_0` or later (sarama's default predates the
produce/fetch v3 message format that supports headers and silently drops
them otherwise).

## See it running

[`catalog-service`](https://github.com/jhonsferg/argos/tree/main/samples/catalog-service)
publishes a `catalog.item.created` event per created item, verified to carry
a `traceparent` header on the real broker.
