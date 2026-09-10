package main

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub/v2"

	argosgcppubsub "github.com/jhonsferg/argos/integrations/gcppubsub"
)

// EventPublisher publishes shipment domain events to GCP Pub/Sub.
type EventPublisher struct {
	publisher *pubsub.Publisher
}

func NewEventPublisher(publisher *pubsub.Publisher) *EventPublisher {
	return &EventPublisher{publisher: publisher}
}

func (p *EventPublisher) PublishShipmentUpdated(ctx context.Context, sh Shipment) error {
	payload, err := json.Marshal(sh)
	if err != nil {
		return err
	}
	_, err = argosgcppubsub.Publish(ctx, p.publisher, &pubsub.Message{Data: payload})
	return err
}
