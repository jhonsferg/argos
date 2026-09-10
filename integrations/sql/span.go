package argossql

import (
	"context"
	"database/sql/driver"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/capture"
)

// startSpan begins a client span for a single database operation (e.g.
// "exec", "query", "prepare"). query is only recorded as an attribute when
// WithQueryText or WithMaskedQueryText was enabled - masked takes
// precedence when both are.
func (c *config) startSpan(ctx context.Context, operation, query string) (context.Context, trace.Span, time.Time) {
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, operation, trace.WithSpanKind(trace.SpanKindClient))

	if c.system.Key != "" {
		span.SetAttributes(c.system)
	}
	if operation != "" {
		span.SetAttributes(semconv.DBOperationName(operation))
	}
	if query != "" {
		switch {
		case c.queryTextMasked:
			span.SetAttributes(semconv.DBQueryText(capture.MaskSQL(query)))
		case c.queryText:
			span.SetAttributes(semconv.DBQueryText(query))
		}
	}
	return ctx, span, time.Now()
}

// endSpan finalizes a span started by startSpan. err == driver.ErrSkip is
// not a real failure (it's the documented "fast path unavailable, caller
// falls back" signal) and is never recorded as a span error.
func (c *config) endSpan(ctx context.Context, span trace.Span, operation string, start time.Time, err error) {
	if err != nil && err != driver.ErrSkip {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if c.logger != nil {
			c.logger.Error(ctx, "database operation failed", err)
		}
	}
	span.End()

	if c.duration != nil {
		attrs := make([]attribute.KeyValue, 0, 2)
		if c.system.Key != "" {
			attrs = append(attrs, c.system)
		}
		if operation != "" {
			attrs = append(attrs, semconv.DBOperationName(operation))
		}
		c.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	}
}
