package argosgcppubsub

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub/v2"
)

// Publish/Receive both require a live *pubsub.Publisher/*pubsub.Subscriber
// (concrete SDK types with no fake-without-a-network-dial construction path
// short of standing up pstest's own gRPC server) - see integration_test.go
// for their real coverage. callHandler is the one piece of this package's
// logic that's pure enough to test directly: the panic-recovery/error
// wiring Receive's per-message callback relies on.

func TestCallHandler_RecoversPanic(t *testing.T) {
	err := callHandler(context.Background(), &pubsub.Message{}, func(context.Context, *pubsub.Message) {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected the panic to be recovered into an error")
	}
}

func TestCallHandler_ReturnsNilOnSuccess(t *testing.T) {
	var called bool
	err := callHandler(context.Background(), &pubsub.Message{}, func(context.Context, *pubsub.Message) {
		called = true
	})
	if err != nil {
		t.Fatalf("callHandler: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
}
