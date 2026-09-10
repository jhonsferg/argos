//go:build integration

package argosrabbitmq_test

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	tcrabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argosrabbitmq "github.com/jhonsferg/argos/integrations/rabbitmq"
)

// TestPublishConsume_RealRabbitMQ proves context propagates from producer to
// consumer across a real broker - the F3 exit criterion.
func TestPublishConsume_RealRabbitMQ(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := tcrabbitmq.Run(ctx, "rabbitmq:3.13-alpine")
	if err != nil {
		t.Fatalf("start rabbitmq container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	amqpURL, err := container.AmqpURL(ctx)
	if err != nil {
		t.Fatalf("AmqpURL: %v", err)
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	q, err := ch.QueueDeclare("orders", false, true, false, false, nil)
	if err != nil {
		t.Fatalf("QueueDeclare: %v", err)
	}

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

	deliveries, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}

	if err := argosrabbitmq.Publish(context.Background(), ch, "", q.Name, false, false, amqp.Publishing{Body: []byte("hello")}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case delivery := <-deliveries:
		if err := argosrabbitmq.Consume(context.Background(), delivery, func(context.Context, amqp.Delivery) error {
			return nil
		}); err != nil {
			t.Fatalf("Consume: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for the published message")
	}

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans (publish, process), got %d", len(spans))
	}
	producerSpan, consumerSpan := spans[0], spans[1]
	if consumerSpan.Parent.TraceID() != producerSpan.SpanContext.TraceID() || consumerSpan.Parent.SpanID() != producerSpan.SpanContext.SpanID() {
		t.Error("consumer span is not a child of the producer span - context did not propagate across the real broker")
	}
}
