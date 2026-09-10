// Package otelcore wires the actual OpenTelemetry SDK providers
// (TracerProvider/MeterProvider/LoggerProvider) from a resolved
// configuration. It is internal: everything outside argos-core reaches this
// only through argos.Init/argos.Provider.
package otelcore

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Params configures Bootstrap. Endpoint/Insecure/Protocol select the OTLP
// exporters for all three signals; the underlying gRPC/HTTP clients dial
// lazily and never block Init on an unreachable collector (fail-open).
type Params struct {
	Endpoint string
	Insecure bool
	GRPC     bool // true: gRPC exporters, false: HTTP exporters
	Resource *resource.Resource
	Sampler  sdktrace.Sampler
}

// Providers bundles the three SDK providers Bootstrap builds.
type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
}

// Bootstrap builds and starts the TracerProvider, MeterProvider, and
// LoggerProvider described by p. It never fails solely because the
// collector at p.Endpoint is unreachable - OTLP client construction here is
// non-blocking; connectivity problems only ever surface as dropped/retried
// exports later.
func Bootstrap(ctx context.Context, p Params) (*Providers, error) {
	traceExp, err := newTraceExporter(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("argos: build trace exporter: %w", err)
	}
	metricExp, err := newMetricExporter(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("argos: build metric exporter: %w", err)
	}
	logExp, err := newLogExporter(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("argos: build log exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(p.Resource),
		sdktrace.WithSampler(p.Sampler),
	)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(p.Resource),
	)

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(p.Resource),
	)

	return &Providers{TracerProvider: tp, MeterProvider: mp, LoggerProvider: lp}, nil
}

func newTraceExporter(ctx context.Context, p Params) (sdktrace.SpanExporter, error) {
	if p.GRPC {
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(p.Endpoint)}
		if p.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)
	}
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(p.Endpoint)}
	if p.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	return otlptracehttp.New(ctx, opts...)
}

func newMetricExporter(ctx context.Context, p Params) (sdkmetric.Exporter, error) {
	if p.GRPC {
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(p.Endpoint)}
		if p.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		return otlpmetricgrpc.New(ctx, opts...)
	}
	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(p.Endpoint)}
	if p.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	return otlpmetrichttp.New(ctx, opts...)
}

func newLogExporter(ctx context.Context, p Params) (sdklog.Exporter, error) {
	if p.GRPC {
		opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(p.Endpoint)}
		if p.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		return otlploggrpc.New(ctx, opts...)
	}
	opts := []otlploghttp.Option{otlploghttp.WithEndpoint(p.Endpoint)}
	if p.Insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}
	return otlploghttp.New(ctx, opts...)
}
