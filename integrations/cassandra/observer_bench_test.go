package argoscassandra_test

import (
	"context"
	"testing"
	"time"

	"github.com/gocql/gocql"

	argoscassandra "github.com/jhonsferg/argos/integrations/cassandra"
)

// BenchmarkObserveQuery_Baseline measures a no-op observer call - the
// "without Argos" comparison point.
func BenchmarkObserveQuery_Baseline(b *testing.B) {
	observe := func(context.Context, gocql.ObservedQuery) {}
	ctx := context.Background()
	q := gocql.ObservedQuery{Keyspace: "orders", Statement: "SELECT * FROM items WHERE id = ?"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		observe(ctx, q)
	}
}

// BenchmarkObserveQuery_Instrumented measures the same call through
// argoscassandra.New, documenting the allocation cost of the retroactive
// span+metric pipeline this package adds per query.
func BenchmarkObserveQuery_Instrumented(b *testing.B) {
	obs := argoscassandra.New()
	ctx := context.Background()
	now := time.Now()
	q := gocql.ObservedQuery{Keyspace: "orders", Statement: "SELECT * FROM items WHERE id = ?", Start: now, End: now}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obs.ObserveQuery(ctx, q)
	}
}
