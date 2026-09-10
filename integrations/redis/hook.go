package argosredis

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/logging"
)

var defaultSystem = semconv.DBSystemRedis

// NewHook builds a redis.Hook. Register it once per client:
//
//	client.AddHook(argosredis.NewHook())
func NewHook(opts ...Option) redis.Hook {
	return &hook{cfg: newConfig(opts...)}
}

type hook struct{ cfg *config }

// DialHook is passed through unchanged - connection-establishment spans are
// out of scope here; every command already gets one via ProcessHook.
func (h *hook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *hook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		operation := cmd.Name()
		ctx, span, start := h.cfg.startSpan(ctx, operation)
		switch {
		case h.cfg.queryTextMasked:
			span.SetAttributes(semconv.DBQueryText(maskCommand(cmd)))
		case h.cfg.queryText:
			span.SetAttributes(semconv.DBQueryText(cmd.String()))
		}

		err := next(ctx, cmd)

		h.cfg.endSpan(ctx, span, operation, start, err)
		return err
	}
}

func (h *hook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		ctx, span, start := h.cfg.startSpan(ctx, "pipeline")
		span.SetAttributes(attribute.Int("db.operation.batch.size", len(cmds)))

		err := next(ctx, cmds)

		h.cfg.endSpan(ctx, span, "pipeline", start, err)
		return err
	}
}

func (c *config) startSpan(ctx context.Context, operation string) (context.Context, trace.Span, time.Time) {
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, operation, trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(c.system, semconv.DBOperationName(operation))
	return ctx, span, time.Now()
}

func (c *config) endSpan(ctx context.Context, span trace.Span, operation string, start time.Time, err error) {
	if err != nil && err != redis.Nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if c.logger != nil {
			c.logger.Error(ctx, "redis command failed", err, logging.F("operation", operation))
		}
	}
	span.End()

	if c.duration != nil {
		c.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(c.system, semconv.DBOperationName(operation)))
	}
}

// maskCommand renders cmd keeping only its command name (cmd.Args()[0]) -
// every argument after it (keys, values, anything else) is redacted, since
// unlike a SQL query's fixed shape, a Redis command's later arguments are
// data values, not structure worth preserving.
func maskCommand(cmd redis.Cmder) string {
	args := cmd.Args()
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	parts[0] = fmt.Sprint(args[0])
	for i := 1; i < len(args); i++ {
		parts[i] = "?"
	}
	return strings.Join(parts, " ")
}
