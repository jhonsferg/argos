// Package argosmongo instruments go.mongodb.org/mongo-driver/v2 via
// event.CommandMonitor - the driver's own hook for every command sent to a
// server. Wire it in via options.Client().SetMonitor(argosmongo.NewMonitor()).
package argosmongo

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/mongo"

// Option configures the monitor returned by NewMonitor.
type Option func(*config)

// WithQueryText enables recording the command as the db.query.text
// attribute. Off by default: a MongoDB command document can embed literal
// argument values (PII, secrets), so this is an explicit opt-in, matching
// argossql.WithQueryText's rationale.
func WithQueryText(enabled bool) Option {
	return func(c *config) { c.queryText = enabled }
}

// WithMaskedQueryText enables recording the command with its argument
// values redacted (top-level field names kept, e.g. "{insert: ?, documents: ?}")
// as the db.query.text attribute - a middle ground between WithQueryText's
// raw capture and no capture at all. Takes precedence over WithQueryText
// if both are enabled, regardless of call order.
func WithMaskedQueryText(enabled bool) Option {
	return func(c *config) { c.queryTextMasked = enabled }
}

// WithLogger attaches a Logger for error-level logging of failed commands,
// in addition to the span/metric recording that always happens.
func WithLogger(l logging.Logger) Option {
	return func(c *config) { c.logger = l }
}

type config struct {
	queryText       bool
	queryTextMasked bool
	logger          logging.Logger
	duration        metric.Float64Histogram
}

func newConfig(opts ...Option) *config {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	if c.logger == nil {
		c.logger = argoslog.Default()
	}
	if h, err := otel.Meter(instrumentationName).Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of MongoDB client operations"),
	); err == nil {
		c.duration = h
	}
	return c
}
