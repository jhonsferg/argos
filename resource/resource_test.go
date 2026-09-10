package resource

import (
	"context"
	"testing"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func TestDetect(t *testing.T) {
	res, err := Detect(context.Background(), "svc", "1.2.3", "staging")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	got := map[string]string{}
	for _, kv := range res.Attributes() {
		got[string(kv.Key)] = kv.Value.String()
	}

	if got[string(semconv.ServiceNameKey)] != "svc" {
		t.Errorf("service.name = %q, want %q", got[string(semconv.ServiceNameKey)], "svc")
	}
	if got[string(semconv.ServiceVersionKey)] != "1.2.3" {
		t.Errorf("service.version = %q, want %q", got[string(semconv.ServiceVersionKey)], "1.2.3")
	}
	if got[string(semconv.DeploymentEnvironmentKey)] != "staging" {
		t.Errorf("deployment.environment = %q, want %q", got[string(semconv.DeploymentEnvironmentKey)], "staging")
	}
}

func TestDetect_OptionalFieldsOmitted(t *testing.T) {
	res, err := Detect(context.Background(), "svc", "", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	for _, kv := range res.Attributes() {
		if kv.Key == semconv.ServiceVersionKey {
			t.Errorf("service.version should be absent when not provided, got %q", kv.Value.String())
		}
		if kv.Key == semconv.DeploymentEnvironmentKey {
			t.Errorf("deployment.environment should be absent when not provided, got %q", kv.Value.String())
		}
	}
}
