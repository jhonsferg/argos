package argos

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

// TestInitShutdown_Smoke exercises Init/Shutdown against the default
// (unreachable in CI) localhost:4317 collector. Per the fail-open design
// (see internal/otelcore), the OTLP client constructors never dial
// synchronously, so Init must succeed and Shutdown must return within the
// bounded context below even with nothing listening on that port.
func TestInitShutdown_Smoke(t *testing.T) {
	provider, err := Init(context.Background(),
		WithServiceName("argos-core-test"),
		WithServiceVersion("0.0.0-test"),
		WithEnvironment("test"),
		WithSampleRatio(1.0),
	)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if provider.Logger() == nil {
		t.Fatal("Provider.Logger() returned nil")
	}

	// A log call must not panic even before Shutdown.
	provider.Logger().Info(context.Background(), "smoke test log line")

	if tp := otel.GetTracerProvider(); tp == nil {
		t.Error("otel.GetTracerProvider() is nil after Init")
	}
	if mp := otel.GetMeterProvider(); mp == nil {
		t.Error("otel.GetMeterProvider() is nil after Init")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- provider.Shutdown(ctx) }()

	select {
	case <-done:
		// Shutdown returning an error here (e.g. failed export flush to an
		// unreachable collector) is expected and not a test failure - what
		// matters is that it returns instead of hanging.
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not return within its context deadline")
	}
}

func TestWithConfig_ThenOptionsOverride(t *testing.T) {
	base := defaultConfig()
	base.ServiceName = "from-config"

	cfg := defaultConfig()
	WithConfig(base)(&cfg)
	WithServiceVersion("9.9.9")(&cfg)

	if cfg.ServiceName != "from-config" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "from-config")
	}
	if cfg.ServiceVersion != "9.9.9" {
		t.Errorf("ServiceVersion = %q, want %q", cfg.ServiceVersion, "9.9.9")
	}
}
