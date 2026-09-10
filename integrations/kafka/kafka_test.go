package argoskafka_test

import (
	"context"
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argoskafka "github.com/jhonsferg/argos/integrations/kafka"
)

type recoveringErrorReporter struct{ t *testing.T }

func (r recoveringErrorReporter) Errorf(format string, args ...any) { r.t.Errorf(format, args...) }

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

func TestSend_InjectsHeadersAndRecordsSpan(t *testing.T) {
	exp := setTracer(t)

	var gotHeaders []sarama.RecordHeader
	producer := mocks.NewSyncProducer(recoveringErrorReporter{t}, mocks.NewTestConfig())
	producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		gotHeaders = msg.Headers
		return nil
	})
	defer func() { _ = producer.Close() }()

	msg := &sarama.ProducerMessage{Topic: "orders", Value: sarama.StringEncoder("hello")}
	if _, _, err := argoskafka.Send(context.Background(), producer, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var found bool
	for _, h := range gotHeaders {
		if string(h.Key) == "traceparent" {
			found = true
		}
	}
	if !found {
		t.Error("expected a traceparent header to be injected into the produced message")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].SpanKind.String() != "producer" {
		t.Errorf("span kind = %v, want producer", spans[0].SpanKind)
	}
}

func TestSend_RecordsErrorOnFailure(t *testing.T) {
	exp := setTracer(t)

	producer := mocks.NewSyncProducer(recoveringErrorReporter{t}, mocks.NewTestConfig())
	producer.ExpectSendMessageAndFail(errors.New("broker unavailable"))
	defer func() { _ = producer.Close() }()

	msg := &sarama.ProducerMessage{Topic: "orders", Value: sarama.StringEncoder("hello")}
	if _, _, err := argoskafka.Send(context.Background(), producer, msg); err == nil {
		t.Fatal("expected an error")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}

func TestConsume_ExtractsContextAndCallsHandler(t *testing.T) {
	exp := setTracer(t)

	// Simulate a message produced with propagation already injected, by
	// round-tripping through Send against a throwaway producer first.
	producer := mocks.NewSyncProducer(recoveringErrorReporter{t}, mocks.NewTestConfig())
	var produced *sarama.ProducerMessage
	producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		produced = msg
		return nil
	})
	if _, _, err := argoskafka.Send(context.Background(), producer, &sarama.ProducerMessage{Topic: "orders"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	_ = producer.Close()
	exp.Reset()

	headers := make([]*sarama.RecordHeader, len(produced.Headers))
	for i := range produced.Headers {
		headers[i] = &produced.Headers[i]
	}
	consumerMsg := &sarama.ConsumerMessage{Topic: "orders", Headers: headers}

	var handlerCalled bool
	err := argoskafka.Consume(context.Background(), consumerMsg, func(ctx context.Context, msg *sarama.ConsumerMessage) error {
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
	if spans[0].SpanKind.String() != "consumer" {
		t.Errorf("span kind = %v, want consumer", spans[0].SpanKind)
	}
	if !spans[0].Parent.IsValid() {
		t.Error("expected the consumer span to have a valid parent from the propagated headers")
	}
}

func TestConsume_RecoversPanicInHandler(t *testing.T) {
	setTracer(t)

	msg := &sarama.ConsumerMessage{Topic: "orders"}
	err := argoskafka.Consume(context.Background(), msg, func(context.Context, *sarama.ConsumerMessage) error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected the panic to be recovered into an error")
	}
}
