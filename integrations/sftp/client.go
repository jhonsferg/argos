package argossftp

import (
	"context"
	"os"
	"time"

	"github.com/pkg/sftp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/logging"
)

// Client wraps *sftp.Client. Every method of the embedded client not
// overridden below is used unmodified via Go's normal method promotion.
type Client struct {
	*sftp.Client
	cfg *config
}

// Wrap builds an instrumented Client around c.
func Wrap(c *sftp.Client, opts ...Option) *Client {
	return &Client{Client: c, cfg: newConfig(opts...)}
}

// OpenContext is Client.Open with a span.
func (c *Client) OpenContext(ctx context.Context, path string) (*sftp.File, error) {
	ctx, span, start := c.start(ctx, "open", path)
	f, err := c.Open(path)
	c.end(ctx, span, "open", start, err)
	return f, err
}

// CreateContext is Client.Create with a span.
func (c *Client) CreateContext(ctx context.Context, path string) (*sftp.File, error) {
	ctx, span, start := c.start(ctx, "create", path)
	f, err := c.Create(path)
	c.end(ctx, span, "create", start, err)
	return f, err
}

// RemoveContext is Client.Remove with a span.
func (c *Client) RemoveContext(ctx context.Context, path string) error {
	ctx, span, start := c.start(ctx, "remove", path)
	err := c.Remove(path)
	c.end(ctx, span, "remove", start, err)
	return err
}

// MkdirContext is Client.Mkdir with a span.
func (c *Client) MkdirContext(ctx context.Context, path string) error {
	ctx, span, start := c.start(ctx, "mkdir", path)
	err := c.Mkdir(path)
	c.end(ctx, span, "mkdir", start, err)
	return err
}

// MkdirAllContext is Client.MkdirAll with a span.
func (c *Client) MkdirAllContext(ctx context.Context, path string) error {
	ctx, span, start := c.start(ctx, "mkdir_all", path)
	err := c.MkdirAll(path)
	c.end(ctx, span, "mkdir_all", start, err)
	return err
}

// RenameContext is Client.Rename with a span. When WithPathAttribute is
// set, the destination is recorded as sftp.destination_path alongside the
// source path recorded on every span.
func (c *Client) RenameContext(ctx context.Context, oldname, newname string) error {
	ctx, span, start := c.start(ctx, "rename", oldname)
	if c.cfg.pathAttribute {
		span.SetAttributes(attribute.String("sftp.destination_path", newname))
	}
	err := c.Rename(oldname, newname)
	c.end(ctx, span, "rename", start, err)
	return err
}

// StatContext is Client.Stat with a span.
func (c *Client) StatContext(ctx context.Context, path string) (os.FileInfo, error) {
	ctx, span, start := c.start(ctx, "stat", path)
	fi, err := c.Stat(path)
	c.end(ctx, span, "stat", start, err)
	return fi, err
}

func (c *Client) start(ctx context.Context, operation, path string) (context.Context, trace.Span, time.Time) {
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, operation, trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(attribute.String("sftp.operation", operation))
	if c.cfg.pathAttribute && path != "" {
		span.SetAttributes(attribute.String("sftp.path", path))
	}
	return ctx, span, time.Now()
}

func (c *Client) end(ctx context.Context, span trace.Span, operation string, start time.Time, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if c.cfg.logger != nil {
			c.cfg.logger.Error(ctx, "sftp operation failed", err, logging.F("operation", operation))
		}
	}
	span.End()

	if c.cfg.duration != nil {
		c.cfg.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("sftp.operation", operation)))
	}
}
