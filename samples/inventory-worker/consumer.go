package main

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"

	argoskafka "github.com/jhonsferg/argos/integrations/kafka"
	argoslog "github.com/jhonsferg/argos/log"
)

// StockChangedTopic must match samples/inventory-api's events.go - this is
// a separate module (each sample is independently runnable), so the shape
// is duplicated rather than shared, same as any two independently
// deployable services agreeing on a wire format.
const StockChangedTopic = "inventory.stock.changed"

// StockChanged mirrors samples/inventory-api's event payload.
type StockChanged struct {
	ItemID    string `json:"item_id"`
	Remaining int    `json:"remaining"`
}

// AuditRecorder is the persistence interface EventProcessor depends on for
// the audit trail. *AuditStore satisfies it; tests use a fake so the
// processing logic is testable without a real MongoDB.
type AuditRecorder interface {
	Record(ctx context.Context, evt StockChanged) error
}

// AnalyticsAppender is the persistence interface EventProcessor depends on
// for the analytics log. *AnalyticsStore satisfies it; tests use a fake so
// the processing logic is testable without a real Cassandra.
type AnalyticsAppender interface {
	Append(ctx context.Context, evt StockChanged) error
}

// EventProcessor is the business logic run per consumed event: record it in
// both the audit store and the analytics store.
type EventProcessor struct {
	audit     AuditRecorder
	analytics AnalyticsAppender
}

func NewEventProcessor(audit AuditRecorder, analytics AnalyticsAppender) *EventProcessor {
	return &EventProcessor{audit: audit, analytics: analytics}
}

func (p *EventProcessor) Process(ctx context.Context, evt StockChanged) error {
	if err := p.audit.Record(ctx, evt); err != nil {
		return err
	}
	return p.analytics.Append(ctx, evt)
}

// consumerGroupHandler adapts EventProcessor to sarama.ConsumerGroupHandler,
// wrapping each message in argoskafka.Consume so the consumer span is a
// child of whatever trace the producer (inventory-api) propagated in the
// message headers.
type consumerGroupHandler struct {
	processor *EventProcessor
}

func (consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		err := argoskafka.Consume(session.Context(), msg, func(ctx context.Context, msg *sarama.ConsumerMessage) error {
			var evt StockChanged
			if err := json.Unmarshal(msg.Value, &evt); err != nil {
				return err
			}
			return h.processor.Process(ctx, evt)
		})
		if err != nil {
			argoslog.Error(session.Context(), "process stock-changed event failed", err, argoslog.F("offset", msg.Offset))
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
