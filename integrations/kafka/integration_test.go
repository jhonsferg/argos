//go:build integration

package argoskafka_test

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argoskafka "github.com/jhonsferg/argos/integrations/kafka"
)

// TestSendConsume_RealKafka proves context propagates from producer to
// consumer across a real broker - the F3 exit criterion.
func TestSendConsume_RealKafka(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := tckafka.Run(ctx, "confluentinc/confluent-local:7.6.1")
	if err != nil {
		t.Fatalf("start kafka container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	brokers, err := container.Brokers(ctx)
	if err != nil {
		t.Fatalf("brokers: %v", err)
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

	cfg := sarama.NewConfig()
	// Headers (needed for trace propagation) require the message format
	// introduced in the produce/fetch v3 protocol - sarama's own default
	// Version predates that and silently drops them without this.
	cfg.Version = sarama.V2_8_0_0
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		t.Fatalf("NewSyncProducer: %v", err)
	}
	defer func() { _ = producer.Close() }()

	const topic = "argos-integration-test"
	if _, _, err := argoskafka.Send(context.Background(), producer, &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder("hello"),
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	consumer, err := sarama.NewConsumer(brokers, cfg)
	if err != nil {
		t.Fatalf("NewConsumer: %v", err)
	}
	defer func() { _ = consumer.Close() }()

	partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		t.Fatalf("ConsumePartition: %v", err)
	}
	defer func() { _ = partitionConsumer.Close() }()

	select {
	case msg := <-partitionConsumer.Messages():
		if err := argoskafka.Consume(context.Background(), msg, func(context.Context, *sarama.ConsumerMessage) error {
			return nil
		}); err != nil {
			t.Fatalf("Consume: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for the produced message")
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
