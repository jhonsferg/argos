# RabbitMQ

`integrations/rabbitmq` wraps `github.com/rabbitmq/amqp091-go`'s
publish/consume as free functions, the same shape as [Kafka](kafka.md).

```go
import (
	amqp "github.com/rabbitmq/amqp091-go"

	argosrabbitmq "github.com/jhonsferg/argos/integrations/rabbitmq"
)

// producer side
err := argosrabbitmq.Publish(ctx, ch, "", "notifications", false, false, amqp.Publishing{
	Body: payload,
})

// consumer side - context propagates from whichever producer sent it
err := argosrabbitmq.Consume(ctx, delivery, func(ctx context.Context, d amqp.Delivery) error {
	return handle(ctx, d)
})
```

## See it running

[`notifications-worker`](https://github.com/jhonsferg/argos/tree/main/samples/notifications-worker)
consumes a `notifications` queue and sends an email per message.
