package main

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"

	argoskafka "github.com/jhonsferg/argos/integrations/kafka"
)

// StockChangedTopic is consumed by samples/inventory-worker.
const StockChangedTopic = "inventory.stock.changed"

// StockChanged is published whenever a reservation succeeds.
type StockChanged struct {
	ItemID    string `json:"item_id"`
	Remaining int    `json:"remaining"`
}

// EventPublisher publishes inventory domain events to Kafka, propagating
// the active trace context in the message headers so inventory-worker's
// consumer span links back to the request that produced it.
type EventPublisher struct {
	producer sarama.SyncProducer
}

func NewEventPublisher(producer sarama.SyncProducer) *EventPublisher {
	return &EventPublisher{producer: producer}
}

func (p *EventPublisher) PublishStockChanged(ctx context.Context, evt StockChanged) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, _, err = argoskafka.Send(ctx, p.producer, &sarama.ProducerMessage{
		Topic: StockChangedTopic,
		Key:   sarama.StringEncoder(evt.ItemID),
		Value: sarama.ByteEncoder(payload),
	})
	return err
}
