package main

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"

	argoskafka "github.com/jhonsferg/argos/integrations/kafka"
)

const itemCreatedTopic = "catalog.item.created"

// EventPublisher publishes catalog domain events to Kafka.
type EventPublisher struct {
	producer sarama.SyncProducer
}

func NewEventPublisher(producer sarama.SyncProducer) *EventPublisher {
	return &EventPublisher{producer: producer}
}

func (p *EventPublisher) PublishItemCreated(ctx context.Context, item Item) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, _, err = argoskafka.Send(ctx, p.producer, &sarama.ProducerMessage{
		Topic: itemCreatedTopic,
		Value: sarama.ByteEncoder(payload),
	})
	return err
}
