package argossmtp

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/smtp"

type Option func(*config)

// WithRecipients records the "to" addresses as a span attribute. Off by
// default since email addresses are personally identifiable information.
func WithRecipients(enabled bool) Option {
	return func(c *config) { c.recipients = enabled }
}

func WithLogger(l logging.Logger) Option {
	return func(c *config) { c.logger = l }
}

type config struct {
	recipients bool
	logger     logging.Logger
	duration   metric.Float64Histogram
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
		"smtp.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of SMTP client operations"),
	); err == nil {
		c.duration = h
	}
	return c
}
