// Package argossftp instruments github.com/pkg/sftp clients. *sftp.Client is
// a concrete struct whose methods (besides ReadDirContext) take no
// context.Context - the library predates that convention, same constraint
// hit with Azure Service Bus's *Sender in the messaging phase. Because it's
// a concrete struct rather than an interface, Wrap can embed it: every
// method not explicitly overridden below (Close, Chmod, Getwd, ...) keeps
// working unmodified through Go's normal method promotion. The overridden
// operations gain *Context-suffixed siblings - mirroring the library's own
// ReadDir/ReadDirContext naming - rather than silently changing an existing
// method's signature.
//
// Neither SFTP nor SMTP (the other niche-phase module) have an
// OpenTelemetry semantic-convention namespace, unlike HTTP/DB/messaging/
// RPC; the attributes here (sftp.operation, sftp.path) are Argos-specific.
package argossftp

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/sftp"

// Option configures the Client returned by Wrap.
type Option func(*config)

// WithPathAttribute enables recording the file path as the sftp.path
// attribute. Off by default: paths can embed usernames or other
// identifying information callers may not want copied into telemetry.
func WithPathAttribute(enabled bool) Option {
	return func(c *config) { c.pathAttribute = enabled }
}

// WithLogger attaches a Logger for error-level logging of failed
// operations, in addition to the span/metric recording that always
// happens.
func WithLogger(l logging.Logger) Option {
	return func(c *config) { c.logger = l }
}

type config struct {
	pathAttribute bool
	logger        logging.Logger
	duration      metric.Float64Histogram
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
		"sftp.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of SFTP client operations"),
	); err == nil {
		c.duration = h
	}
	return c
}
