package argosazuresb

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	messagingcore "github.com/jhonsferg/argos/integrations/messaging/core"
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

// TestStartSend_InjectsPropertiesAndRecordsSpan covers everything
// SendMessage does except the literal network call to a live Sender (see
// the package doc for why that call itself can't be unit tested).
func TestStartSend_InjectsPropertiesAndRecordsSpan(t *testing.T) {
	exp := setTracer(t)
	instr := messagingcore.New()

	msg := &azservicebus.Message{Body: []byte("hello")}
	if msg.ApplicationProperties != nil {
		t.Fatal("test setup: expected a nil ApplicationProperties map")
	}

	_, span, _ := startSend(context.Background(), msg, instr)
	span.End()

	if msg.ApplicationProperties == nil {
		t.Fatal("expected startSend to initialize ApplicationProperties")
	}
	if _, ok := msg.ApplicationProperties["traceparent"]; !ok {
		t.Error("expected a traceparent application property to be injected")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].SpanKind.String() != "producer" {
		t.Errorf("span kind = %v, want producer", spans[0].SpanKind)
	}
}

func TestProcess_ExtractsPropagatedContext(t *testing.T) {
	exp := setTracer(t)

	msg := &azservicebus.ReceivedMessage{
		ApplicationProperties: map[string]any{
			"traceparent": "00-11111111111111111111111111111111-2222222222222222-01",
		},
	}

	var handlerCalled bool
	err := Process(context.Background(), msg, func(ctx context.Context, m *azservicebus.ReceivedMessage) error {
		handlerCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if !spans[0].Parent.IsValid() {
		t.Error("expected the consumer span to have a valid parent from the traceparent property")
	}
	if spans[0].SpanKind.String() != "consumer" {
		t.Errorf("span kind = %v, want consumer", spans[0].SpanKind)
	}
}

func TestProcess_RecoversPanicInHandler(t *testing.T) {
	setTracer(t)

	err := Process(context.Background(), &azservicebus.ReceivedMessage{}, func(context.Context, *azservicebus.ReceivedMessage) error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected the panic to be recovered into an error")
	}
}

func TestProcess_RecordsErrorFromHandler(t *testing.T) {
	exp := setTracer(t)

	err := Process(context.Background(), &azservicebus.ReceivedMessage{}, func(context.Context, *azservicebus.ReceivedMessage) error {
		return context.DeadlineExceeded
	})
	if err == nil {
		t.Fatal("expected the handler's error to propagate")
	}
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}
