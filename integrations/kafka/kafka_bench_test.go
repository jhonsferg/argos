package argoskafka_test

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"

	argoskafka "github.com/jhonsferg/argos/integrations/kafka"
)

type noopErrorReporter struct{}

func (noopErrorReporter) Errorf(string, ...any) {}

// BenchmarkSend_Baseline measures the fake producer directly - the "without
// Argos" comparison point.
func BenchmarkSend_Baseline(b *testing.B) {
	producer := mocks.NewSyncProducer(noopErrorReporter{}, mocks.NewTestConfig())
	defer func() { _ = producer.Close() }()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		producer.ExpectSendMessageAndSucceed()
		if _, _, err := producer.SendMessage(&sarama.ProducerMessage{Topic: "orders", Value: sarama.StringEncoder("hello")}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSend_Instrumented measures the same producer through
// argoskafka.Send, documenting the allocation cost of the span+metric
// pipeline and header injection this package adds per message.
func BenchmarkSend_Instrumented(b *testing.B) {
	producer := mocks.NewSyncProducer(noopErrorReporter{}, mocks.NewTestConfig())
	defer func() { _ = producer.Close() }()
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		producer.ExpectSendMessageAndSucceed()
		if _, _, err := argoskafka.Send(ctx, producer, &sarama.ProducerMessage{Topic: "orders", Value: sarama.StringEncoder("hello")}); err != nil {
			b.Fatal(err)
		}
	}
}
