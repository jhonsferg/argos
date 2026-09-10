// Package argosgcppubsub instruments cloud.google.com/go/pubsub/v2
// publishers and subscribers (the v1 cloud.google.com/go/pubsub package is
// deprecated upstream in favor of v2, so this targets v2 directly).
// Message.Attributes is already exactly map[string]string, so it needs no
// adapter type - otel's own propagation.MapCarrier wraps it directly.
package argosgcppubsub

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	messagingcore "github.com/jhonsferg/argos/integrations/messaging/core"
	"github.com/jhonsferg/argos/middleware"
)

// Option re-exports messagingcore.Option so callers only need this package.
type Option = messagingcore.Option

// WithLogger re-exports messagingcore.WithLogger.
var WithLogger = messagingcore.WithLogger

// Publish injects the active trace context from ctx into msg.Attributes
// (initializing the map if nil), then publishes msg via publisher and -
// unlike calling publisher.Publish directly - waits for the result before
// returning, so the PRODUCER span this records actually reflects
// success/failure and real duration instead of just enqueue time. This
// trades away independent async batching from the caller's perspective for
// a correct span; call publisher.Publish directly (after injecting
// yourself) if that trade-off doesn't fit your use case.
func Publish(ctx context.Context, publisher *pubsub.Publisher, msg *pubsub.Message, opts ...Option) (serverID string, err error) {
	if msg.Attributes == nil {
		msg.Attributes = map[string]string{}
	}
	instr := messagingcore.New(opts...)
	spanCtx, span := instr.StartProducer(ctx, semconv.MessagingSystemGCPPubsub, publisher.ID(), propagation.MapCarrier(msg.Attributes))
	start := time.Now()

	serverID, err = publisher.Publish(ctx, msg).Get(ctx)

	instr.End(spanCtx, span, semconv.MessagingSystemGCPPubsub, "publish", start, err)
	return serverID, err
}

// Receive wraps subscriber.Receive: for every message, it extracts any
// propagated trace context from msg.Attributes, starts a CONSUMER span, and
// calls handler within it before returning control to the pubsub client
// library (handler remains responsible for calling msg.Ack()/Nack() as
// usual). A panic inside handler is recovered (via core/middleware.Handle,
// recording it on the span) rather than crashing the receive loop.
func Receive(ctx context.Context, subscriber *pubsub.Subscriber, handler func(context.Context, *pubsub.Message), opts ...Option) error {
	instr := messagingcore.New(opts...)
	return subscriber.Receive(ctx, func(msgCtx context.Context, msg *pubsub.Message) {
		spanCtx, span := instr.StartConsumer(msgCtx, semconv.MessagingSystemGCPPubsub, subscriber.ID(), propagation.MapCarrier(msg.Attributes))
		start := time.Now()

		err := callHandler(spanCtx, msg, handler)

		instr.End(spanCtx, span, semconv.MessagingSystemGCPPubsub, "process", start, err)
	})
}

func callHandler(ctx context.Context, msg *pubsub.Message, handler func(context.Context, *pubsub.Message)) (err error) {
	defer func() {
		if panicErr := middleware.Handle(ctx, nil, recover()); panicErr != nil {
			err = panicErr
		}
	}()
	handler(ctx, msg)
	return nil
}
