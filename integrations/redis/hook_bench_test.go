package argosredis_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	argosredis "github.com/jhonsferg/argos/integrations/redis"
)

// BenchmarkSet_Baseline measures a client with no hook registered - the
// "without Argos" comparison point.
func BenchmarkSet_Baseline(b *testing.B) {
	mr := miniredis.RunT(b)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSet_Instrumented measures the same client with the Hook
// registered, documenting the allocation cost it adds per command.
func BenchmarkSet_Instrumented(b *testing.B) {
	mr := miniredis.RunT(b)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	client.AddHook(argosredis.NewHook())
	defer func() { _ = client.Close() }()
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
			b.Fatal(err)
		}
	}
}
