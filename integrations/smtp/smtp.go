// Package argossmtp instruments net/smtp.SendMail, the standard library's
// (frozen, feature-complete) low-level SMTP client. SendMail is a
// package-level function, not a method on some interface or struct, so this
// wraps it as a free function taking a context.Context, the same shape used
// for the other concrete-SDK systems in this repo.
package argossmtp

import (
	"context"
	"net"
	"net/smtp"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/logging"
)

// SendMail is net/smtp.SendMail wrapped in a CLIENT span and duration
// metric. Arguments and behavior are otherwise identical to the stdlib
// function.
func SendMail(ctx context.Context, addr string, a smtp.Auth, from string, to []string, msg []byte, opts ...Option) error {
	cfg := newConfig(opts...)
	ctx, span, start := start(ctx, addr, to, cfg)

	err := smtp.SendMail(addr, a, from, to, msg)

	end(ctx, span, start, err, cfg)
	return err
}

func start(ctx context.Context, addr string, to []string, cfg *config) (context.Context, trace.Span, time.Time) {
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, "send", trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(attribute.String("smtp.operation", "send"))

	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	span.SetAttributes(semconv.ServerAddress(host))

	if cfg.recipients && len(to) > 0 {
		span.SetAttributes(attribute.StringSlice("smtp.recipients", to))
	}
	return ctx, span, time.Now()
}

func end(ctx context.Context, span trace.Span, start time.Time, err error, cfg *config) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if cfg.logger != nil {
			cfg.logger.Error(ctx, "smtp send failed", err, logging.F("operation", "send"))
		}
	}
	span.End()

	if cfg.duration != nil {
		cfg.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("smtp.operation", "send")))
	}
}
