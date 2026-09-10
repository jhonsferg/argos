//go:build integration

package argosgcppubsub_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	tcpubsub "github.com/testcontainers/testcontainers-go/modules/gcloud/pubsub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	argosgcppubsub "github.com/jhonsferg/argos/integrations/gcppubsub"
)

// TestPublishReceive_RealEmulator proves context propagates from publisher
// to subscriber across a real (emulated) Pub/Sub service - the F3 exit
// criterion.
func TestPublishReceive_RealEmulator(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := tcpubsub.Run(ctx, "gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators")
	if err != nil {
		t.Fatalf("start pubsub emulator container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	projectID := container.ProjectID()
	client, err := pubsub.NewClient(ctx, projectID,
		option.WithEndpoint(container.URI()),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

	topicName := fmt.Sprintf("projects/%s/topics/orders", projectID)
	if _, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topicName}); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	subName := fmt.Sprintf("projects/%s/subscriptions/orders-sub", projectID)
	if _, err := client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{Name: subName, Topic: topicName}); err != nil {
		t.Fatalf("CreateSubscription: %v", err)
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

	publisher := client.Publisher(topicName)
	defer publisher.Stop()

	if _, err := argosgcppubsub.Publish(context.Background(), publisher, &pubsub.Message{Data: []byte("hello")}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	subscriber := client.Subscriber(subName)
	receiveCtx, receiveCancel := context.WithCancel(ctx)
	defer receiveCancel()

	// Pub/Sub is at-least-once delivery: the emulator may (and did, in
	// practice) redeliver before this test's single Ack is fully processed,
	// so this only stops accepting new deliveries after the first one -
	// it does not assume exactly one.
	var once sync.Once
	done := make(chan struct{})
	go func() {
		_ = argosgcppubsub.Receive(receiveCtx, subscriber, func(_ context.Context, msg *pubsub.Message) {
			msg.Ack()
			once.Do(func() { close(done) })
		})
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for the published message to be received")
	}
	receiveCancel()

	var producerSpan *tracetest.SpanStub
	var propagated bool
	for i, span := range exp.GetSpans() {
		if span.SpanKind.String() == "producer" {
			producerSpan = &exp.GetSpans()[i]
			continue
		}
		if producerSpan != nil && span.Parent.TraceID() == producerSpan.SpanContext.TraceID() && span.Parent.SpanID() == producerSpan.SpanContext.SpanID() {
			propagated = true
		}
	}
	if producerSpan == nil {
		t.Fatal("expected at least one producer span")
	}
	if !propagated {
		t.Error("no consumer span was a child of the producer span - context did not propagate across the real emulator")
	}
}
