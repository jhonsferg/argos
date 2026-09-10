package argossql_test

import (
	"context"
	"database/sql"
	"testing"

	argossql "github.com/jhonsferg/argos/integrations/sql"
)

// BenchmarkExec_Baseline measures the fake modern driver directly, with no
// Argos wrapping - the "without Argos" comparison point.
func BenchmarkExec_Baseline(b *testing.B) {
	db, err := sql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (1)"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExec_Instrumented measures the same driver opened through
// argossql.Open, documenting the allocation cost of the span+metric
// pipeline this package adds per query.
func BenchmarkExec_Instrumented(b *testing.B) {
	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (1)"); err != nil {
			b.Fatal(err)
		}
	}
}
