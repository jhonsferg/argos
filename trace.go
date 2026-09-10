package argos

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	argoslog "github.com/jhonsferg/argos/log"
)

const traceInstrumentationName = "github.com/jhonsferg/argos"

var (
	traceDurationOnce sync.Once
	traceDuration     metric.Float64Histogram
)

func traceDurationHistogram() metric.Float64Histogram {
	traceDurationOnce.Do(func() {
		if h, err := otel.Meter(traceInstrumentationName).Float64Histogram(
			"operation.duration",
			metric.WithUnit("s"),
			metric.WithDescription("Duration of operations wrapped by argos.Trace/TraceFunc"),
		); err == nil {
			traceDuration = h
		}
	})
	return traceDuration
}

// Trace runs fn inside a new INTERNAL span named name, recording fn's error
// (if any) on the span, through core/log's global Logger, and returns fn's
// result unchanged. It is the one-line equivalent of manually calling
// tracer.Start/span.RecordError/span.End around arbitrary business logic -
// no struct field, no per-return-shape constructor needed:
//
//	order, err := argos.Trace(ctx, "orders.FindByID", func(ctx context.Context) (*Order, error) {
//		return repo.findByID(ctx, id)
//	})
//
// Callers that need to add attributes inside fn can still reach the active
// span via trace.SpanFromContext(ctx).
func Trace[T any](ctx context.Context, name string, fn func(context.Context) (T, error)) (T, error) {
	tracer := otel.Tracer(traceInstrumentationName)
	ctx, span := tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	start := time.Now()

	result, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		argoslog.Error(ctx, "operation failed", err, argoslog.F("operation", name))
	}
	if h := traceDurationHistogram(); h != nil {
		h.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("operation", name)))
	}
	return result, err
}

// TraceFunc is Trace for operations with no return value besides error.
func TraceFunc(ctx context.Context, name string, fn func(context.Context) error) error {
	_, err := Trace(ctx, name, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}
