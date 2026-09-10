// Package messagingcore holds the span/metric logic shared by every Argos
// messaging adapter (kafka, rabbitmq, azuresb, gcppubsub). Each adapter
// provides only a propagation.TextMapCarrier over its own system's message
// header/property type and a thin wrapper matching that system's client
// shape - the actual span/metric bookkeeping lives here, written once.
package messagingcore

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/messaging/core"

// Option configures an Instrumentor.
type Option func(*Instrumentor)

// WithLogger attaches a Logger for error-level logging of failed
// produce/consume operations, in addition to the span/metric recording that
// always happens.
func WithLogger(l logging.Logger) Option {
	return func(i *Instrumentor) { i.logger = l }
}

// Instrumentor holds the (created-once) duration histogram and
// configuration shared across every produce/consume call built from it.
type Instrumentor struct {
	logger   logging.Logger
	duration metric.Float64Histogram
}

// New builds an Instrumentor. Create one per process (or per client) and
// reuse it - it owns the duration histogram, which must not be recreated
// per message.
func New(opts ...Option) *Instrumentor {
	i := &Instrumentor{}
	for _, opt := range opts {
		opt(i)
	}
	if i.logger == nil {
		i.logger = argoslog.Default()
	}
	if h, err := otel.Meter(instrumentationName).Float64Histogram(
		"messaging.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of messaging client operations"),
	); err == nil {
		i.duration = h
	}
	return i
}

// StartProducer begins a PRODUCER span for a message about to be sent to
// destination, then injects the resulting context into carrier - built over
// the outgoing message's own header/property map - so a consumer on the
// other end of the wire can continue the same trace.
func (i *Instrumentor) StartProducer(ctx context.Context, system attribute.KeyValue, destination string, carrier propagation.TextMapCarrier) (context.Context, trace.Span) {
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, "publish", trace.WithSpanKind(trace.SpanKindProducer))
	span.SetAttributes(system, semconv.MessagingDestinationName(destination), semconv.MessagingOperationName("publish"))
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return ctx, span
}

// StartConsumer begins a CONSUMER span for a message received from
// destination, extracting any propagated context from carrier first so the
// span becomes a child of whatever producer sent it.
func (i *Instrumentor) StartConsumer(ctx context.Context, system attribute.KeyValue, destination string, carrier propagation.TextMapCarrier) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, "process", trace.WithSpanKind(trace.SpanKindConsumer))
	span.SetAttributes(system, semconv.MessagingDestinationName(destination), semconv.MessagingOperationName("process"))
	return ctx, span
}

// End finalizes a span started by StartProducer/StartConsumer: records the
// duration histogram, marks the span an error (and logs it, if a Logger is
// configured) when err is non-nil, and ends the span.
func (i *Instrumentor) End(ctx context.Context, span trace.Span, system attribute.KeyValue, operation string, start time.Time, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if i.logger != nil {
			i.logger.Error(ctx, "messaging operation failed", err, logging.F("operation", operation))
		}
	}
	span.End()

	if i.duration != nil {
		i.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(system, semconv.MessagingOperationName(operation)))
	}
}
