// Package argosazuresb instruments Azure Service Bus
// (.../sdk/messaging/azservicebus) senders and receivers. Sender is a
// concrete SDK type with no live-connection-free way to construct or fake
// it (unlike Kafka's mocks or a hand-built struct), so the actual
// SendMessage network call has no fast unit test - see azuresb_test.go for
// how the injection logic around it (the actual value this package adds)
// is still fully covered by testing startSend directly. Process needs no
// such split: ReceivedMessage is a plain struct, so it's testable end to
// end without any live broker.
package argosazuresb

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	messagingcore "github.com/jhonsferg/argos/integrations/messaging/core"
	"github.com/jhonsferg/argos/middleware"
)

// Option re-exports messagingcore.Option so callers only need this package.
type Option = messagingcore.Option

// WithLogger re-exports messagingcore.WithLogger.
var WithLogger = messagingcore.WithLogger

// SendMessage sends msg via sender, injecting the active trace context from
// ctx into msg.ApplicationProperties first (initializing the map if nil) so
// a receiver on the other end continues the same trace, and records a
// PRODUCER span/metric around the call.
func SendMessage(ctx context.Context, sender *azservicebus.Sender, msg *azservicebus.Message, opts ...Option) error {
	instr := messagingcore.New(opts...)
	spanCtx, span, start := startSend(ctx, msg, instr)

	err := sender.SendMessage(spanCtx, msg, nil)

	instr.End(spanCtx, span, semconv.MessagingSystemServicebus, "publish", start, err)
	return err
}

func startSend(ctx context.Context, msg *azservicebus.Message, instr *messagingcore.Instrumentor) (context.Context, trace.Span, time.Time) {
	if msg.ApplicationProperties == nil {
		msg.ApplicationProperties = map[string]any{}
	}
	spanCtx, span := instr.StartProducer(ctx, semconv.MessagingSystemServicebus, "", propertiesCarrier{props: msg.ApplicationProperties})
	return spanCtx, span, time.Now()
}

// Process extracts any propagated trace context from
// msg.ApplicationProperties, starts a CONSUMER span, and calls handler
// within it. A panic inside handler is recovered (via
// core/middleware.Handle, recording it on the span) and returned as an
// error rather than crashing the caller's receive loop.
func Process(ctx context.Context, msg *azservicebus.ReceivedMessage, handler func(context.Context, *azservicebus.ReceivedMessage) error, opts ...Option) error {
	instr := messagingcore.New(opts...)
	carrier := propertiesCarrier{props: msg.ApplicationProperties}
	spanCtx, span := instr.StartConsumer(ctx, semconv.MessagingSystemServicebus, "", carrier)
	start := time.Now()

	err := callHandler(spanCtx, msg, handler)

	instr.End(spanCtx, span, semconv.MessagingSystemServicebus, "process", start, err)
	return err
}

func callHandler(ctx context.Context, msg *azservicebus.ReceivedMessage, handler func(context.Context, *azservicebus.ReceivedMessage) error) (err error) {
	defer func() {
		if panicErr := middleware.Handle(ctx, nil, recover()); panicErr != nil {
			err = panicErr
		}
	}()
	return handler(ctx, msg)
}
