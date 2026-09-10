// Package argossql instruments any database/sql driver generically via
// database/sql/driver, so the same wrapper works for Postgres, MySQL,
// SQLite, SQL Server, etc. without per-driver code. See doc.go for the
// design notes on how it covers both the modern context-aware driver
// interfaces and the legacy fallback path database/sql itself uses.
package argossql

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/sql"

// Option configures the instrumentation applied by Wrap/WrapConnector/Open.
type Option func(*config)

// WithSystem sets the db.system attribute (e.g. semconv.DBSystemPostgreSQL),
// recorded on every span and metric this package produces. Omitting it just
// means spans/metrics won't carry a db.system attribute - instrumentation
// still works.
func WithSystem(system attribute.KeyValue) Option {
	return func(c *config) { c.system = system }
}

// WithQueryText enables recording the query text as the db.query.text
// attribute. Off by default: query text can embed literal parameter values
// (PII, secrets), so this is an explicit opt-in, not a default-on attribute.
func WithQueryText(enabled bool) Option {
	return func(c *config) { c.queryText = enabled }
}

// WithMaskedQueryText enables recording the query text with literal values
// redacted (see capture.MaskSQL) as the db.query.text attribute - a middle
// ground between WithQueryText's raw capture and no capture at all: the
// query's shape stays visible for debugging without copying parameter
// values into telemetry. Takes precedence over WithQueryText if both are
// enabled, regardless of call order.
func WithMaskedQueryText(enabled bool) Option {
	return func(c *config) { c.queryTextMasked = enabled }
}

// WithLogger attaches a Logger for error-level logging of query failures, in
// addition to the span/metric recording that always happens.
func WithLogger(l logging.Logger) Option {
	return func(c *config) { c.logger = l }
}

type config struct {
	system          attribute.KeyValue
	queryText       bool
	queryTextMasked bool
	logger          logging.Logger
	duration        metric.Float64Histogram
}

// withConfig reuses an already-built config wholesale - used internally to
// propagate a driver's config to the connectors/conns it produces without
// rebuilding the duration histogram for each one.
func withConfig(existing *config) Option {
	return func(c *config) { *c = *existing }
}

func newConfig(opts ...Option) *config {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	if c.logger == nil {
		c.logger = argoslog.Default()
	}
	if c.duration != nil {
		// Propagated from withConfig - don't recreate the instrument.
		return c
	}
	if h, err := otel.Meter(instrumentationName).Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of database client operations"),
	); err == nil {
		c.duration = h
	}
	return c
}
