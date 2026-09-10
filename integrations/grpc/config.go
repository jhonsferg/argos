// Package argosgrpc instruments google.golang.org/grpc servers and clients
// via all four interceptor shapes (unary/stream x server/client).
// Propagation travels in gRPC metadata via a small carrier (carrier.go).
package argosgrpc

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/jhonsferg/argos/capture"
	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/grpc"

// Option configures the interceptors built by this package.
type Option func(*config)

// WithLogger attaches a Logger so panics recovered on the server side (and
// every failed call) are also logged, not just recorded on the span.
func WithLogger(l logging.Logger) Option {
	return func(c *config) { c.logger = l }
}

// WithCapture configures request/response body and metadata capture on
// the client interceptors (UnaryClientInterceptor/StreamClientInterceptor
// - server-side interceptors don't use this) per rules - see
// capture.HTTPRules. Metadata stands in for HTTP headers; the marshaled
// message (via protojson, when it implements proto.Message) stands in for
// the body. Off by default - see capture.go for the exact attribute names.
func WithCapture(rules capture.HTTPRules) Option {
	return func(c *config) { c.rules = rules }
}

type config struct {
	logger   logging.Logger
	duration metric.Float64Histogram
	rules    capture.HTTPRules
}

func newConfig(metricName string, opts ...Option) *config {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	if c.logger == nil {
		c.logger = argoslog.Default()
	}
	if h, err := otel.Meter(instrumentationName).Float64Histogram(
		metricName,
		metric.WithUnit("s"),
		metric.WithDescription("Duration of gRPC calls"),
	); err == nil {
		c.duration = h
	}
	return c
}
