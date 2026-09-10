package argosgrpc_test

import (
	"context"
	"testing"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/jhonsferg/argos/capture"
	argosgrpc "github.com/jhonsferg/argos/integrations/grpc"
)

func TestUnaryClient_WithCapture_BodyOnSuccess(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t, argosgrpc.WithCapture(capture.HTTPRules{
		RequestBody:  capture.Rule{Enabled: true, MaxBytes: 4096},
		ResponseBody: capture.Rule{Enabled: true, MaxBytes: 4096},
	}))

	resp, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: "widget"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("status = %v, want SERVING", resp.Status)
	}

	clientSpan := spanByKind(t, exp.GetSpans(), "client")
	var gotReq, gotResp bool
	for _, kv := range clientSpan.Attributes {
		switch string(kv.Key) {
		case "argos.rpc.request.body":
			if contains(kv.Value.AsString(), "widget") {
				gotReq = true
			}
		case "argos.rpc.response.body":
			if contains(kv.Value.AsString(), "SERVING") {
				gotResp = true
			}
		}
	}
	if !gotReq {
		t.Error("expected argos.rpc.request.body attribute")
	}
	if !gotResp {
		t.Error("expected argos.rpc.response.body attribute")
	}
}

func TestUnaryClient_WithCapture_BodyOnErrorOnly_NotCapturedOnSuccess(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t, argosgrpc.WithCapture(capture.HTTPRules{
		RequestBody:  capture.Rule{Enabled: true, MaxBytes: 4096, OnErrorOnly: true},
		ResponseBody: capture.Rule{Enabled: true, MaxBytes: 4096, OnErrorOnly: true},
	}))

	if _, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check: %v", err)
	}

	clientSpan := spanByKind(t, exp.GetSpans(), "client")
	for _, kv := range clientSpan.Attributes {
		if string(kv.Key) == "argos.rpc.request.body" || string(kv.Key) == "argos.rpc.response.body" {
			t.Errorf("unexpected %s on a successful call with OnErrorOnly set", kv.Key)
		}
	}
}

func TestUnaryClient_WithCapture_BodyOnErrorOnly_CapturedOnFailure(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t, argosgrpc.WithCapture(capture.HTTPRules{
		RequestBody: capture.Rule{Enabled: true, MaxBytes: 4096, OnErrorOnly: true},
	}))

	if _, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: "boom"}); err == nil {
		t.Fatal("expected an error from the \"boom\" service")
	}

	clientSpan := spanByKind(t, exp.GetSpans(), "client")
	var found bool
	for _, kv := range clientSpan.Attributes {
		if string(kv.Key) == "argos.rpc.request.body" {
			found = true
		}
	}
	if !found {
		t.Error("expected argos.rpc.request.body attribute on a failed call with OnErrorOnly set")
	}
}

func TestUnaryClient_WithCapture_MetadataExcludeAndMask(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t, argosgrpc.WithCapture(capture.HTTPRules{
		RequestHeaders: capture.HeaderRule{
			Enabled: true,
			Exclude: []string{"authorization"},
			Mask:    []string{"x-api-key"},
		},
	}))

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer secret", "x-api-key", "key-123", "x-plain", "visible")
	if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check: %v", err)
	}

	clientSpan := spanByKind(t, exp.GetSpans(), "client")
	attrs := map[string]string{}
	for _, kv := range clientSpan.Attributes {
		if vals := kv.Value.AsStringSlice(); len(vals) > 0 {
			attrs[string(kv.Key)] = vals[0]
		}
	}
	if _, ok := attrs["rpc.grpc.request.metadata.authorization"]; ok {
		t.Error("authorization must never be captured (Exclude)")
	}
	if got := attrs["rpc.grpc.request.metadata.x-api-key"]; got != "***" {
		t.Errorf("x-api-key = %q, want masked", got)
	}
	if got := attrs["rpc.grpc.request.metadata.x-plain"]; got != "visible" {
		t.Errorf("x-plain = %q, want unmasked", got)
	}
}

func TestUnaryClient_NoCaptureConfigured_NoAttributes(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t) // no WithCapture

	if _, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: "widget"}); err != nil {
		t.Fatalf("Check: %v", err)
	}

	clientSpan := spanByKind(t, exp.GetSpans(), "client")
	for _, kv := range clientSpan.Attributes {
		t.Logf("attr: %s", kv.Key)
		if string(kv.Key) == "argos.rpc.request.body" || string(kv.Key) == "argos.rpc.response.body" {
			t.Errorf("unexpected %s when WithCapture was never configured", kv.Key)
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
