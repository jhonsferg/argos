package argosgrpc_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	core "github.com/jhonsferg/argos"
	argosgrpc "github.com/jhonsferg/argos/integrations/grpc"
)

func TestUnaryClient_WithYAMLConfig_MetadataCapture(t *testing.T) {
	exp := setTracer(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n" +
		"  grpc:\n" +
		"    capture:\n" +
		"      request_headers:\n" +
		"        enabled: true\n" +
		"        exclude: [\"authorization\"]\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := core.FromYAML(path)
	if err != nil {
		t.Fatalf("FromYAML: %v", err)
	}

	client := dialClient(t, argosgrpc.WithYAMLConfig(cfg))

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer secret", "x-plain", "visible")
	if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check: %v", err)
	}

	clientSpan := spanByKind(t, exp.GetSpans(), "client")
	var gotPlain bool
	for _, kv := range clientSpan.Attributes {
		switch string(kv.Key) {
		case "rpc.grpc.request.metadata.authorization":
			t.Error("authorization must not be captured (excluded via YAML)")
		case "rpc.grpc.request.metadata.x-plain":
			gotPlain = true
		}
	}
	if !gotPlain {
		t.Error("expected x-plain to be captured per the YAML-configured rule")
	}
}
