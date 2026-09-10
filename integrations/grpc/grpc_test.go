package argosgrpc_test

import (
	"context"
	"net"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	argosgrpc "github.com/jhonsferg/argos/integrations/grpc"
)

// healthServer implements healthpb.HealthServer minimally: Check succeeds
// unless asked about "boom" (returns an error) or "panic" (panics), and
// Watch sends one response before returning - enough to exercise both the
// unary and streaming server interceptors.
type healthServer struct {
	healthpb.UnimplementedHealthServer
}

func (healthServer) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	switch req.Service {
	case "boom":
		return nil, grpcstatus.Error(grpccodes.Internal, "boom")
	case "panic":
		panic("kaboom")
	default:
		return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
	}
}

func (healthServer) Watch(req *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	return stream.Send(&healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING})
}

// spanByKind finds the (single) span of the given kind. The server span
// finishes before the client span in these in-process tests (the server
// handler returns, ending its span, before the client's invoker call
// returns), so tests must not assume export order.
func spanByKind(t *testing.T, spans tracetest.SpanStubs, kind string) tracetest.SpanStub {
	t.Helper()
	for _, s := range spans {
		if s.SpanKind.String() == kind {
			return s
		}
	}
	t.Fatalf("no span with kind %q found among %d spans", kind, len(spans))
	return tracetest.SpanStub{}
}

func setTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})
	return exp
}

// dialClient starts a real grpc.Server over an in-memory bufconn listener
// with Argos's server interceptors installed, and returns a HealthClient
// dialed through Argos's client interceptors - a full, real client->server
// round trip, no external process or container needed.
func dialClient(t *testing.T, clientOpts ...argosgrpc.Option) healthpb.HealthClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(argosgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(argosgrpc.StreamServerInterceptor()),
	)
	healthpb.RegisterHealthServer(srv, healthServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(argosgrpc.UnaryClientInterceptor(clientOpts...)),
		grpc.WithStreamInterceptor(argosgrpc.StreamClientInterceptor()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn)
}

func TestUnary_ServerSpanIsChildOfClientSpan(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t)

	resp, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans (client, server), got %d", len(spans))
	}
	clientSpan := spanByKind(t, spans, "client")
	serverSpan := spanByKind(t, spans, "server")
	if serverSpan.Parent.TraceID() != clientSpan.SpanContext.TraceID() || serverSpan.Parent.SpanID() != clientSpan.SpanContext.SpanID() {
		t.Error("server span is not a child of the client span - metadata propagation failed")
	}
}

func TestUnary_RecordsGRPCError(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t)

	_, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: "boom"})
	if err == nil {
		t.Fatal("expected an error")
	}

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	for _, span := range spans {
		if span.Status.Code != codes.Error {
			t.Errorf("span %q status = %v, want %v", span.Name, span.Status.Code, codes.Error)
		}
	}
}

func TestUnary_RecoversPanicAsInternalError(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t)

	_, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: "panic"})
	if err == nil {
		t.Fatal("expected an error from the recovered panic")
	}
	if grpcstatus.Code(err) != grpccodes.Internal {
		t.Errorf("code = %v, want %v", grpcstatus.Code(err), grpccodes.Internal)
	}

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
}

func TestStream_ServerSpanIsChildOfClientSpan(t *testing.T) {
	exp := setTracer(t)
	client := dialClient(t)

	stream, err := client.Watch(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("expected EOF after the single response")
	}

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans (client, server), got %d", len(spans))
	}
	clientSpan := spanByKind(t, spans, "client")
	serverSpan := spanByKind(t, spans, "server")
	if serverSpan.Parent.TraceID() != clientSpan.SpanContext.TraceID() || serverSpan.Parent.SpanID() != clientSpan.SpanContext.SpanID() {
		t.Error("server span is not a child of the client span - metadata propagation failed")
	}
}
