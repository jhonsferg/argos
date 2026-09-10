// Package argosgorm instruments GORM via its Plugin interface, hooking the
// same before/after callback pairs gorm.io/plugin/opentelemetry uses
// (Create/Query/Update/Delete/Row/Raw). It's a peer of argos-sql, not a
// replacement for it: this package adds ORM-level spans (operation, table),
// while opening the underlying *sql.DB through argossql.Open additionally
// instruments the actual driver round trip - see the integration test for a
// worked example combining both.
package argosgorm

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/gorm"

// Option configures the Plugin returned by New.
type Option func(*config)

// WithSystem sets the db.system attribute (e.g. semconv.DBSystemPostgreSQL)
// recorded on every span/metric this package produces.
func WithSystem(system attribute.KeyValue) Option {
	return func(c *config) { c.system = system }
}

// WithQueryText enables recording the built SQL as the db.query.text
// attribute. Off by default - see argossql.WithQueryText for why.
func WithQueryText(enabled bool) Option {
	return func(c *config) { c.queryText = enabled }
}

// WithMaskedQueryText enables recording the built SQL with literal values
// redacted (see capture.MaskSQL) as the db.query.text attribute - a middle
// ground between WithQueryText's raw capture and no capture at all. Takes
// precedence over WithQueryText if both are enabled, regardless of call
// order.
func WithMaskedQueryText(enabled bool) Option {
	return func(c *config) { c.queryTextMasked = enabled }
}

// WithLogger attaches a Logger for error-level logging of failed operations,
// in addition to the span/metric recording that always happens.
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
		metric.WithDescription("Duration of GORM operations"),
	); err == nil {
		c.duration = h
	}
	return c
}
