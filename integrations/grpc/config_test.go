package argosgrpc_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	argosgrpc "github.com/jhonsferg/argos/integrations/grpc"
	argoslogging "github.com/jhonsferg/argos/logging"
)

// TestWithLogger_LogsOnPanicRecovery uses its own bufconn server (rather
// than the shared dialClient helper) because WithLogger only has an effect
// on the server-side panic recovery path (see coremiddleware.Handle in
// grpc.go), and dialClient's server interceptor is wired with no options.
func TestWithLogger_LogsOnPanicRecovery(t *testing.T) {
	setTracer(t)
	logs := &capturingLogger{}

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(grpc.UnaryInterceptor(argosgrpc.UnaryServerInterceptor(argosgrpc.WithLogger(logs))))
	healthpb.RegisterHealthServer(srv, healthServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := healthpb.NewHealthClient(conn)
	if _, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: "panic"}); err == nil {
		t.Fatal("expected an error from the recovered panic")
	}
	if logs.errorCalls == 0 {
		t.Error("expected at least 1 Error log call for the recovered panic")
	}
}

// capturingLogger is a minimal logging.Logger test double that only counts
// Error calls.
type capturingLogger struct{ errorCalls int }

func (l *capturingLogger) Debug(context.Context, string, ...argoslogging.KV) {}
func (l *capturingLogger) Info(context.Context, string, ...argoslogging.KV)  {}
func (l *capturingLogger) Warn(context.Context, string, ...argoslogging.KV)  {}
func (l *capturingLogger) Error(context.Context, string, error, ...argoslogging.KV) {
	l.errorCalls++
}
func (l *capturingLogger) With(...argoslogging.KV) argoslogging.Logger { return l }
