// Package argosrabbitmq instruments github.com/rabbitmq/amqp091-go
// producers and consumers. Both Channel.PublishWithContext and Delivery are
// concrete types, not interfaces, so - like argos-kafka - this is a pair of
// free functions rather than a wrapped client, built on messagingcore.
package argosrabbitmq

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	messagingcore "github.com/jhonsferg/argos/integrations/messaging/core"
	"github.com/jhonsferg/argos/middleware"
)

// Option re-exports messagingcore.Option so callers only need this package.
type Option = messagingcore.Option

// WithLogger re-exports messagingcore.WithLogger.
var WithLogger = messagingcore.WithLogger

// Publish sends msg via ch.PublishWithContext, injecting the active trace
// context from ctx into msg.Headers first (initializing the table if nil)
// so a consumer on the other end continues the same trace, and records a
// PRODUCER span/metric around the call.
func Publish(ctx context.Context, ch *amqp.Channel, exchange, routingKey string, mandatory, immediate bool, msg amqp.Publishing, opts ...Option) error {
	if msg.Headers == nil {
		msg.Headers = amqp.Table{}
	}
	destination := exchange
	if destination == "" {
		destination = routingKey
	}

	instr := messagingcore.New(opts...)
	spanCtx, span := instr.StartProducer(ctx, semconv.MessagingSystemRabbitmq, destination, tableCarrier{table: msg.Headers})
	span.SetAttributes(semconv.MessagingRabbitmqDestinationRoutingKey(routingKey))
	start := time.Now()

	err := ch.PublishWithContext(ctx, exchange, routingKey, mandatory, immediate, msg)

	instr.End(spanCtx, span, semconv.MessagingSystemRabbitmq, "publish", start, err)
	return err
}

// Consume extracts any propagated trace context from delivery.Headers,
// starts a CONSUMER span, and calls handler within it. A panic inside
// handler is recovered (via core/middleware.Handle, recording it on the
// span) and returned as an error rather than crashing the caller's consume
// loop.
func Consume(ctx context.Context, delivery amqp.Delivery, handler func(context.Context, amqp.Delivery) error, opts ...Option) error {
	instr := messagingcore.New(opts...)
	carrier := tableCarrier{table: delivery.Headers}
	spanCtx, span := instr.StartConsumer(ctx, semconv.MessagingSystemRabbitmq, delivery.RoutingKey, carrier)
	start := time.Now()

	err := callHandler(spanCtx, delivery, handler)

	instr.End(spanCtx, span, semconv.MessagingSystemRabbitmq, "process", start, err)
	return err
}

func callHandler(ctx context.Context, delivery amqp.Delivery, handler func(context.Context, amqp.Delivery) error) (err error) {
	defer func() {
		if panicErr := middleware.Handle(ctx, nil, recover()); panicErr != nil {
			err = panicErr
		}
	}()
	return handler(ctx, delivery)
}
