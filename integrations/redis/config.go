// Package argosredis instruments go-redis (v9) clients via redis.Hook -
// register NewHook's result with client.AddHook(...) to get a span per
// command (or per pipeline) plus a client operation duration metric.
package argosredis

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/redis"

// Option configures the Hook returned by NewHook.
type Option func(*config)

// WithSystem overrides the db.system attribute. Defaults to
// semconv.DBSystemRedis - only override this for a Redis-protocol-compatible
// store you want labeled differently.
func WithSystem(system attribute.KeyValue) Option {
	return func(c *config) { c.system = system }
}

// WithQueryText enables recording the formatted command as the
// db.query.text attribute. Off by default - command arguments can contain
// values you don't want copied into telemetry.
func WithQueryText(enabled bool) Option {
	return func(c *config) { c.queryText = enabled }
}

// WithMaskedQueryText enables recording the command with its arguments
// redacted (command name kept, every argument replaced with "?") as the
// db.query.text attribute - a middle ground between WithQueryText's raw
// capture and no capture at all. Takes precedence over WithQueryText if
// both are enabled, regardless of call order.
func WithMaskedQueryText(enabled bool) Option {
	return func(c *config) { c.queryTextMasked = enabled }
}

// WithLogger attaches a Logger for error-level logging of failed commands,
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
	if c.system.Key == "" {
		c.system = defaultSystem
	}
	if h, err := otel.Meter(instrumentationName).Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of Redis client operations"),
	); err == nil {
		c.duration = h
	}
	return c
}
