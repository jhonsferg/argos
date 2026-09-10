// Package resource builds the OpenTelemetry Resource describing this
// service instance. It is computed once at Init() time and cached - never
// recomputed per request.
package resource

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Detect builds a *resource.Resource carrying service.name, service.version,
// deployment.environment, plus host/OS/process detectors and any attributes
// supplied via the OTEL_RESOURCE_ATTRIBUTES/OTEL_SERVICE_NAME environment
// variables (resource.WithFromEnv). It runs once during Init(); callers must
// not call it per request.
func Detect(ctx context.Context, serviceName, serviceVersion, environment string) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
	}
	if serviceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(serviceVersion))
	}
	if environment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironment(environment))
	}

	detected, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithContainer(),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("argos: detect resource: %w", err)
	}

	// Merge in the SDK's own default resource (telemetry.sdk.name/version/
	// language) so those attributes are present even though they aren't
	// covered by any detector above. Attributes from `detected` win on
	// conflict (e.g. service.name).
	res, err := resource.Merge(resource.Default(), detected)
	if err != nil {
		return nil, fmt.Errorf("argos: merge resource: %w", err)
	}
	return res, nil
}
