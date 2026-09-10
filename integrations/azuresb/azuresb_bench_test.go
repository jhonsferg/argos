package argosazuresb_test

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	argosazuresb "github.com/jhonsferg/argos/integrations/azuresb"
)

func noopHandler(context.Context, *azservicebus.ReceivedMessage) error { return nil }

// BenchmarkProcess_Baseline measures calling the handler directly - the
// "without Argos" comparison point.
func BenchmarkProcess_Baseline(b *testing.B) {
	msg := &azservicebus.ReceivedMessage{}
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := noopHandler(ctx, msg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProcess_Instrumented measures the same handler called through
// argosazuresb.Process, documenting the allocation cost of the span+metric
// pipeline and property extraction this package adds per message.
func BenchmarkProcess_Instrumented(b *testing.B) {
	msg := &azservicebus.ReceivedMessage{}
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := argosazuresb.Process(ctx, msg, noopHandler); err != nil {
			b.Fatal(err)
		}
	}
}
