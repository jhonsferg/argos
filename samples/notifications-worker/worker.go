package main

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"

	argos "github.com/jhonsferg/argos"
	argosrabbitmq "github.com/jhonsferg/argos/integrations/rabbitmq"
)

// Notification is the JSON body of each message on the queue.
type Notification struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// Worker consumes deliveries from a RabbitMQ queue and sends an email per
// message.
type Worker struct {
	channel *amqp.Channel
	mailer  Mailer
	logger  argos.Logger
}

func NewWorker(channel *amqp.Channel, mailer Mailer, logger argos.Logger) *Worker {
	return &Worker{channel: channel, mailer: mailer, logger: logger}
}

// Run consumes queue until ctx is cancelled or the delivery channel closes.
// Each delivery is processed through argosrabbitmq.Consume so the consumer
// span is linked to whichever producer span sent the message.
func (w *Worker) Run(ctx context.Context, queue string) error {
	deliveries, err := w.channel.Consume(queue, "", true, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return nil
			}
			if err := argosrabbitmq.Consume(ctx, delivery, w.handle); err != nil {
				w.logger.Error(ctx, "processing notification failed", err)
			}
		}
	}
}

func (w *Worker) handle(ctx context.Context, delivery amqp.Delivery) error {
	var n Notification
	if err := json.Unmarshal(delivery.Body, &n); err != nil {
		return err
	}
	return w.mailer.Send(ctx, n)
}
