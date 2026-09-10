// Package argoscassandra instruments github.com/gocql/gocql via its
// QueryObserver/BatchObserver hooks. Unlike Mongo's CommandMonitor, each
// hook fires exactly once, after the query already completed, with its
// Start/End timestamps and error included - so spans here are built
// retroactively via trace.WithTimestamp rather than needing before/after
// correlation. Wire it in via cluster.QueryObserver = argoscassandra.New(...)
// and cluster.BatchObserver = <same value> (New returns something that
// implements both).
package argoscassandra

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/cassandra"

// Option configures the Observer returned by New.
type Option func(*config)

// WithQueryText enables recording the statement as the db.query.text
// attribute. Off by default: bound values can embed literal parameter
// values (PII, secrets), so this is an explicit opt-in, matching
// argossql.WithQueryText's rationale.
func WithQueryText(enabled bool) Option {
	return func(c *config) { c.queryText = enabled }
}

// WithMaskedQueryText enables recording the statement with literal values
// redacted (see capture.MaskSQL) as the db.query.text attribute - a middle
// ground between WithQueryText's raw capture and no capture at all. Takes
// precedence over WithQueryText if both are enabled, regardless of call
// order.
func WithMaskedQueryText(enabled bool) Option {
	return func(c *config) { c.queryTextMasked = enabled }
}

// WithLogger attaches a Logger for error-level logging of failed
// queries/batches, in addition to the span/metric recording that always
// happens.
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
		metric.WithDescription("Duration of Cassandra client operations"),
	); err == nil {
		c.duration = h
	}
	return c
}
