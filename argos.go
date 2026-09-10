// Package argos is the entry point of argos-core: a single Init() wires
// tracing, metrics, and correlated logging behind one Provider, using the
// OpenTelemetry SDK underneath.
package argos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel"

	"github.com/jhonsferg/argos/internal/otelcore"
	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
	"github.com/jhonsferg/argos/propagation"
	"github.com/jhonsferg/argos/resource"
)

// Logger re-exports logging.Logger so consumers only need to import this
// package for the common case.
type Logger = logging.Logger

// KV re-exports logging.KV; F re-exports logging.F.
type KV = logging.KV

var F = logging.F

// Provider holds the SDK providers Init built and the Logger derived from
// them. It is the handle used for Shutdown.
type Provider struct {
	providers       *otelcore.Providers
	logger          Logger
	shutdownTimeout time.Duration
}

// Logger returns the configured Logger. Every log call made through it is
// automatically correlated with the trace/span active in the passed
// context.
func (p *Provider) Logger() Logger { return p.logger }

// Shutdown flushes and stops every provider. It should be deferred right
// after a successful Init. Shutdown never panics; it aggregates and returns
// every provider's shutdown error.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error
	if err := p.providers.TracerProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("tracer provider: %w", err))
	}
	if err := p.providers.MeterProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("meter provider: %w", err))
	}
	if err := p.providers.LoggerProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("logger provider: %w", err))
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("argos: shutdown: %w", errors.Join(errs...))
}

// Init builds and installs the TracerProvider, MeterProvider, and
// LoggerProvider globally (traces/metrics via otel.SetTracerProvider /
// otel.SetMeterProvider, plus the W3C tracecontext+baggage propagator via
// otel.SetTextMapPropagator) and returns a Provider handle. Init never
// fails solely because the configured OTLP collector is unreachable - see
// internal/otelcore for why - it only fails on invalid configuration.
func Init(ctx context.Context, opts ...Option) (*Provider, error) {
	cfg := defaultConfig()
	fromEnv(&cfg)
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.err != nil {
		return nil, cfg.err
	}

	res, err := resource.Detect(ctx, cfg.ServiceName, cfg.ServiceVersion, cfg.Environment)
	if err != nil {
		return nil, err
	}

	sampler := cfg.Sampler
	if sampler == nil {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))
	}

	providers, err := otelcore.Bootstrap(ctx, otelcore.Params{
		Endpoint: cfg.Endpoint,
		Insecure: cfg.Insecure,
		GRPC:     cfg.Protocol == ProtocolGRPC,
		Resource: res,
		Sampler:  sampler,
	})
	if err != nil {
		return nil, err
	}

	otel.SetTracerProvider(providers.TracerProvider)
	otel.SetMeterProvider(providers.MeterProvider)
	otel.SetTextMapPropagator(propagation.New())

	logger := cfg.Logger
	if logger == nil {
		otelLogger := providers.LoggerProvider.Logger(cfg.ServiceName)
		logger = logging.NewZerolog(os.Stdout, zerolog.InfoLevel, otelLogger)
	}
	argoslog.SetDefault(logger)

	return &Provider{
		providers:       providers,
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}
