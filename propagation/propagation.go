// Package propagation builds the composite context propagator Argos
// installs globally during Init(). Integrations (HTTP servers/clients, gRPC,
// messaging) read it back via otel.GetTextMapPropagator() - this package
// exists only to define it in one place.
package propagation

import "go.opentelemetry.io/otel/propagation"

// New returns the W3C Trace Context + Baggage composite propagator Argos
// uses by default.
func New() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
