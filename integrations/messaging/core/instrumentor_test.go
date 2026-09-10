package messagingcore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	messagingcore "github.com/jhonsferg/argos/integrations/messaging/core"
)

func setUp(t *testing.T) *tracetest.InMemoryExporter {
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

func TestProducerConsumer_ContextPropagates(t *testing.T) {
	exp := setUp(t)
	instr := messagingcore.New()

	headers := map[string]string{}
	carrier := propagation.MapCarrier(headers)

	pctx, pspan := instr.StartProducer(context.Background(), semconv.MessagingSystemKafka, "orders", carrier)
	instr.End(pctx, pspan, semconv.MessagingSystemKafka, "publish", time.Now(), nil)

	if len(headers) == 0 {
		t.Fatal("expected StartProducer to inject propagation headers")
	}

	cctx, cspan := instr.StartConsumer(context.Background(), semconv.MessagingSystemKafka, "orders", carrier)
	instr.End(cctx, cspan, semconv.MessagingSystemKafka, "process", time.Now(), nil)

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	producerSpan, consumerSpan := spans[0], spans[1]
	if consumerSpan.Parent.TraceID() != producerSpan.SpanContext.TraceID() {
		t.Errorf("consumer span trace ID = %v, want producer's %v", consumerSpan.Parent.TraceID(), producerSpan.SpanContext.TraceID())
	}
	if consumerSpan.Parent.SpanID() != producerSpan.SpanContext.SpanID() {
		t.Errorf("consumer span parent ID = %v, want producer span ID %v", consumerSpan.Parent.SpanID(), producerSpan.SpanContext.SpanID())
	}
}

func TestEnd_RecordsError(t *testing.T) {
	exp := setUp(t)
	instr := messagingcore.New()

	ctx, span := instr.StartProducer(context.Background(), semconv.MessagingSystemRabbitmq, "q", propagation.MapCarrier{})
	instr.End(ctx, span, semconv.MessagingSystemRabbitmq, "publish", time.Now(), errors.New("broker unavailable"))

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}
