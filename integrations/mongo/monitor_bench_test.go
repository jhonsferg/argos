package argosmongo_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/event"

	argosmongo "github.com/jhonsferg/argos/integrations/mongo"
)

// BenchmarkMonitor_Baseline measures calling the driver's own zero-value
// (no-op) monitor hooks - the "without Argos" comparison point.
func BenchmarkMonitor_Baseline(b *testing.B) {
	mon := &event.CommandMonitor{
		Started:   func(context.Context, *event.CommandStartedEvent) {},
		Succeeded: func(context.Context, *event.CommandSucceededEvent) {},
	}
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mon.Started(ctx, &event.CommandStartedEvent{CommandName: "find", DatabaseName: "orders", RequestID: int64(i)})
		mon.Succeeded(ctx, &event.CommandSucceededEvent{
			CommandFinishedEvent: event.CommandFinishedEvent{CommandName: "find", DatabaseName: "orders", RequestID: int64(i)},
		})
	}
}

// BenchmarkMonitor_Instrumented measures the same start/succeed pair through
// argosmongo.NewMonitor, documenting the allocation cost of the span+metric
// pipeline and RequestID correlation this package adds per command.
func BenchmarkMonitor_Instrumented(b *testing.B) {
	mon := argosmongo.NewMonitor()
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mon.Started(ctx, &event.CommandStartedEvent{CommandName: "find", DatabaseName: "orders", RequestID: int64(i)})
		mon.Succeeded(ctx, &event.CommandSucceededEvent{
			CommandFinishedEvent: event.CommandFinishedEvent{CommandName: "find", DatabaseName: "orders", RequestID: int64(i)},
		})
	}
}
