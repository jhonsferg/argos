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
)

func dialClientForBench(b *testing.B, instrumented bool) healthpb.HealthClient {
	b.Helper()
	lis := bufconn.Listen(1024 * 1024)

	var serverOpts []grpc.ServerOption
	var dialOpts []grpc.DialOption
	if instrumented {
		serverOpts = append(serverOpts,
			grpc.UnaryInterceptor(argosgrpc.UnaryServerInterceptor()),
			grpc.StreamInterceptor(argosgrpc.StreamServerInterceptor()),
		)
		dialOpts = append(dialOpts,
			grpc.WithUnaryInterceptor(argosgrpc.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(argosgrpc.StreamClientInterceptor()),
		)
	}

	srv := grpc.NewServer(serverOpts...)
	healthpb.RegisterHealthServer(srv, healthServer{})
	go func() { _ = srv.Serve(lis) }()
	b.Cleanup(srv.Stop)

	dialOpts = append(dialOpts,
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	conn, err := grpc.NewClient("passthrough:///bufnet", dialOpts...)
	if err != nil {
		b.Fatalf("grpc.NewClient: %v", err)
	}
	b.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn)
}

// BenchmarkCheck_Baseline measures a unary call with no Argos interceptors
// installed - the "without Argos" comparison point.
func BenchmarkCheck_Baseline(b *testing.B) {
	client := dialClientForBench(b, false)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCheck_Instrumented measures the same call with Argos's unary
// server and client interceptors installed, documenting the allocation
// cost they add per call.
func BenchmarkCheck_Instrumented(b *testing.B) {
	client := dialClientForBench(b, true)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
			b.Fatal(err)
		}
	}
}
