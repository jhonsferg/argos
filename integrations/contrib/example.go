// Package argoscontrib is a worked template for instrumenting a
// third-party client library, not a real integration. It covers the
// simplest of the three shapes this repo's integrations take: a library
// that already exposes an interface and a context.Context-taking method,
// with no cross-process propagation to worry about. See README.md for the
// decision tree pointing at the other two shapes, each backed by a real
// module in this repo.
package argoscontrib

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/contrib"

// WidgetClient stands in for a third-party client library: an interface,
// with a method that already takes context.Context. Real libraries with
// this shape include most gRPC-generated clients and many REST SDKs.
type WidgetClient interface {
	DoThing(ctx context.Context, name string) error
}

type Option func(*config)

// WithLogger attaches a logger for recording failed operations, matching
// the WithLogger convention used by every other module in this repo.
func WithLogger(l logging.Logger) Option {
	return func(c *config) { c.logger = l }
}

type config struct {
	logger logging.Logger
}

type wrapped struct {
	next     WidgetClient
	cfg      *config
	duration metric.Float64Histogram
}

// Wrap returns a WidgetClient that instruments next with a CLIENT span and
// duration metric per call. Because WidgetClient is an interface, Wrap can
// return one directly - callers depend on the interface, never the
// concrete type, so nothing about their code changes besides the
// constructor call.
func Wrap(next WidgetClient, opts ...Option) WidgetClient {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.logger == nil {
		cfg.logger = argoslog.Default()
	}
	w := &wrapped{next: next, cfg: cfg}
	if h, err := otel.Meter(instrumentationName).Float64Histogram(
		"widget.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of WidgetClient operations"),
	); err == nil {
		w.duration = h
	}
	return w
}

func (w *wrapped) DoThing(ctx context.Context, name string) error {
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, "DoThing", trace.WithSpanKind(trace.SpanKindClient))
	start := time.Now()

	err := w.next.DoThing(ctx, name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if w.cfg.logger != nil {
			w.cfg.logger.Error(ctx, "widget operation failed", err, logging.F("operation", "DoThing"))
		}
	}
	span.End()

	if w.duration != nil {
		w.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("widget.operation", "DoThing")))
	}
	return err
}
