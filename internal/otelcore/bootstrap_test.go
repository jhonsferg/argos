package otelcore

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TestBootstrap_ProtocolCombinations exercises every GRPC/Insecure
// combination Bootstrap can be given. OTLP exporter construction here is
// non-blocking by design (see the package doc comment) - it never dials
// out, so this runs without a real collector and without hanging even
// against the unreachable endpoint below.
func TestBootstrap_ProtocolCombinations(t *testing.T) {
	cases := []struct {
		name     string
		grpc     bool
		insecure bool
	}{
		{"grpc insecure", true, true},
		{"grpc secure", true, false},
		{"http insecure", false, true},
		{"http secure", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			providers, err := Bootstrap(context.Background(), Params{
				Endpoint: "localhost:0",
				Insecure: tc.insecure,
				GRPC:     tc.grpc,
				Resource: resource.Default(),
				Sampler:  sdktrace.AlwaysSample(),
			})
			if err != nil {
				t.Fatalf("Bootstrap: %v", err)
			}
			if providers.TracerProvider == nil || providers.MeterProvider == nil || providers.LoggerProvider == nil {
				t.Fatal("Bootstrap returned a Providers with a nil field")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 0)
			defer cancel()
			_ = providers.TracerProvider.Shutdown(ctx)
			_ = providers.MeterProvider.Shutdown(ctx)
			_ = providers.LoggerProvider.Shutdown(ctx)
		})
	}
}
