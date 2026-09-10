package argossftp_test

import (
	"context"
	"testing"

	argossftp "github.com/jhonsferg/argos/integrations/sftp"
)

// BenchmarkOpen_Baseline measures the underlying *sftp.Client directly -
// the "without Argos" comparison point.
func BenchmarkOpen_Baseline(b *testing.B) {
	client := newInMemoryClient(b)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, err := client.Create("/bench.txt")
		if err != nil {
			b.Fatal(err)
		}
		_ = f.Close()
	}
}

// BenchmarkOpen_Instrumented measures the same operation through
// argossftp.Wrap, documenting the allocation cost of the span+metric
// pipeline this package adds per call.
func BenchmarkOpen_Instrumented(b *testing.B) {
	client := argossftp.Wrap(newInMemoryClient(b))
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, err := client.CreateContext(ctx, "/bench.txt")
		if err != nil {
			b.Fatal(err)
		}
		_ = f.Close()
	}
}
