package argosrabbitmq_test

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argosrabbitmq "github.com/jhonsferg/argos/integrations/rabbitmq"
)

func setTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
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
	return exp
}

// TestConsume_ExtractsPropagatedContext builds a delivery carrying a
// hand-crafted traceparent header (as a real producer's Publish would have
// set it) and verifies Consume's span becomes its child - this is the
// extraction half of context propagation, tested without needing a live
// broker; Publish's injection half is covered by the //go:build integration
// test, since amqp.Channel isn't an interface a fake can stand in for.
func TestConsume_ExtractsPropagatedContext(t *testing.T) {
	exp := setTracer(t)

	delivery := amqp.Delivery{
		RoutingKey: "orders.created",
		Headers:    amqp.Table{"traceparent": "00-11111111111111111111111111111111-2222222222222222-01"},
	}

	var handlerCalled bool
	err := argosrabbitmq.Consume(context.Background(), delivery, func(ctx context.Context, d amqp.Delivery) error {
		handlerCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if !spans[0].Parent.IsValid() {
		t.Error("expected the consumer span to have a valid parent from the traceparent header")
	}
	if spans[0].SpanKind.String() != "consumer" {
		t.Errorf("span kind = %v, want consumer", spans[0].SpanKind)
	}
}

func TestConsume_RecoversPanicInHandler(t *testing.T) {
	setTracer(t)

	err := argosrabbitmq.Consume(context.Background(), amqp.Delivery{}, func(context.Context, amqp.Delivery) error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected the panic to be recovered into an error")
	}
}
