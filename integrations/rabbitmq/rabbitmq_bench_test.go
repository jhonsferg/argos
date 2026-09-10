package argosrabbitmq_test

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	argosrabbitmq "github.com/jhonsferg/argos/integrations/rabbitmq"
)

func noopHandler(context.Context, amqp.Delivery) error { return nil }

// BenchmarkConsume_Baseline measures calling the handler directly - the
// "without Argos" comparison point.
func BenchmarkConsume_Baseline(b *testing.B) {
	delivery := amqp.Delivery{RoutingKey: "orders.created"}
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := noopHandler(ctx, delivery); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConsume_Instrumented measures the same handler called through
// argosrabbitmq.Consume, documenting the allocation cost of the span+metric
// pipeline and context extraction this package adds per message.
func BenchmarkConsume_Instrumented(b *testing.B) {
	delivery := amqp.Delivery{RoutingKey: "orders.created"}
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := argosrabbitmq.Consume(ctx, delivery, noopHandler); err != nil {
			b.Fatal(err)
		}
	}
}
