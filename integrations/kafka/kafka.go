// Package argoskafka instruments github.com/IBM/sarama producers and
// consumers. sarama.SyncProducer's methods take no context.Context (it
// predates that convention), so instead of wrapping the interface - which
// would leave every produced span an unparented root span - this package
// exposes free functions that take an explicit ctx and call through to the
// caller's own producer/message, injecting/extracting propagation headers
// and delegating span/metric bookkeeping to messagingcore.
package argoskafka

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	messagingcore "github.com/jhonsferg/argos/integrations/messaging/core"
	"github.com/jhonsferg/argos/middleware"
)

// Option re-exports messagingcore.Option so callers only need this package.
type Option = messagingcore.Option

// WithLogger re-exports messagingcore.WithLogger.
var WithLogger = messagingcore.WithLogger

// Send publishes msg via p, injecting the active trace context from ctx
// into msg.Headers first so a consumer on the other end continues the same
// trace, and records a PRODUCER span/metric around the call.
func Send(ctx context.Context, p sarama.SyncProducer, msg *sarama.ProducerMessage, opts ...Option) (partition int32, offset int64, err error) {
	instr := messagingcore.New(opts...)
	carrier := producerCarrier{headers: &msg.Headers}
	spanCtx, span := instr.StartProducer(ctx, semconv.MessagingSystemKafka, msg.Topic, carrier)
	start := time.Now()

	partition, offset, err = p.SendMessage(msg)

	instr.End(spanCtx, span, semconv.MessagingSystemKafka, "publish", start, err)
	return partition, offset, err
}

// SendBatch publishes msgs via p as one batch operation: each message gets
// the trace context injected into its own headers (so every one of them
// individually propagates correctly), and the whole call is covered by a
// single span carrying the batch size, matching semconv guidance to use
// messaging.batch.message_count rather than one span per batched message.
func SendBatch(ctx context.Context, p sarama.SyncProducer, msgs []*sarama.ProducerMessage, opts ...Option) error {
	if len(msgs) == 0 {
		return p.SendMessages(msgs)
	}

	instr := messagingcore.New(opts...)
	spanCtx, span := instr.StartProducer(ctx, semconv.MessagingSystemKafka, msgs[0].Topic, producerCarrier{headers: &msgs[0].Headers})
	for _, msg := range msgs[1:] {
		otel.GetTextMapPropagator().Inject(spanCtx, producerCarrier{headers: &msg.Headers})
	}
	span.SetAttributes(semconv.MessagingBatchMessageCount(len(msgs)))
	start := time.Now()

	err := p.SendMessages(msgs)

	instr.End(spanCtx, span, semconv.MessagingSystemKafka, "publish", start, err)
	return err
}

// Consume extracts any propagated trace context from msg.Headers, starts a
// CONSUMER span, and calls handler within it. A panic inside handler is
// recovered (via core/middleware.Handle, recording it on the span) and
// returned as an error rather than crashing the caller's consume loop.
func Consume(ctx context.Context, msg *sarama.ConsumerMessage, handler func(context.Context, *sarama.ConsumerMessage) error, opts ...Option) error {
	instr := messagingcore.New(opts...)
	carrier := consumerCarrier{headers: msg.Headers}
	spanCtx, span := instr.StartConsumer(ctx, semconv.MessagingSystemKafka, msg.Topic, carrier)
	start := time.Now()

	err := callHandler(spanCtx, msg, handler)

	instr.End(spanCtx, span, semconv.MessagingSystemKafka, "process", start, err)
	return err
}

func callHandler(ctx context.Context, msg *sarama.ConsumerMessage, handler func(context.Context, *sarama.ConsumerMessage) error) (err error) {
	defer func() {
		if panicErr := middleware.Handle(ctx, nil, recover()); panicErr != nil {
			err = panicErr
		}
	}()
	return handler(ctx, msg)
}
