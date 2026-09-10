package argosgrpc

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jhonsferg/argos/logging"
)

func (c *config) startServer(ctx context.Context, fullMethod string) (context.Context, trace.Span, time.Time) {
	md, _ := metadata.FromIncomingContext(ctx)
	ctx = otel.GetTextMapPropagator().Extract(ctx, mdCarrier(md))

	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, fullMethod, trace.WithSpanKind(trace.SpanKindServer))
	setRPCAttrs(span, fullMethod)
	return ctx, span, time.Now()
}

func (c *config) startClient(ctx context.Context, fullMethod string) (context.Context, trace.Span, time.Time) {
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, fullMethod, trace.WithSpanKind(trace.SpanKindClient))
	setRPCAttrs(span, fullMethod)

	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		md = md.Copy()
	} else {
		md = metadata.MD{}
	}
	otel.GetTextMapPropagator().Inject(ctx, mdCarrier(md))
	ctx = metadata.NewOutgoingContext(ctx, md)

	return ctx, span, time.Now()
}

func (c *config) end(ctx context.Context, span trace.Span, fullMethod string, err error, start time.Time) {
	span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(status.Code(err))))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		if c.logger != nil {
			c.logger.Error(ctx, "grpc call failed", err, logging.F("method", fullMethod))
		}
	}
	span.End()

	if c.duration != nil {
		c.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(semconv.RPCSystemGRPC))
	}
}

func setRPCAttrs(span trace.Span, fullMethod string) {
	span.SetAttributes(semconv.RPCSystemGRPC)
	service, method := splitFullMethod(fullMethod)
	if service != "" {
		span.SetAttributes(semconv.RPCService(service))
	}
	if method != "" {
		span.SetAttributes(semconv.RPCMethod(method))
	}
}

// splitFullMethod splits a gRPC FullMethod ("/pkg.Service/Method") into its
// service and method parts.
func splitFullMethod(fullMethod string) (service, method string) {
	trimmed := strings.TrimPrefix(fullMethod, "/")
	i := strings.LastIndex(trimmed, "/")
	if i < 0 {
		return "", trimmed
	}
	return trimmed[:i], trimmed[i+1:]
}
