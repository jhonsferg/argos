package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	argos "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/logging"
)

type fakeMailer struct {
	sent []Notification
	err  error
}

func (m *fakeMailer) Send(_ context.Context, n Notification) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, n)
	return nil
}

func testLogger() argos.Logger {
	return logging.NewZerolog(bytes.NewBuffer(nil), 0, nil)
}

func TestWorker_HandleSendsEmail(t *testing.T) {
	mailer := &fakeMailer{}
	w := NewWorker(nil, mailer, testLogger())

	delivery := amqp.Delivery{Body: []byte(`{"to":"a@example.com","subject":"hi","body":"hello"}`)}
	if err := w.handle(context.Background(), delivery); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if len(mailer.sent) != 1 || mailer.sent[0].To != "a@example.com" {
		t.Errorf("unexpected sent notifications: %+v", mailer.sent)
	}
}

func TestWorker_HandleInvalidJSON(t *testing.T) {
	w := NewWorker(nil, &fakeMailer{}, testLogger())

	delivery := amqp.Delivery{Body: []byte("not json")}
	if err := w.handle(context.Background(), delivery); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestWorker_HandlePropagatesMailerError(t *testing.T) {
	mailer := &fakeMailer{err: errors.New("smtp down")}
	w := NewWorker(nil, mailer, testLogger())

	delivery := amqp.Delivery{Body: []byte(`{"to":"a@example.com","subject":"hi","body":"hello"}`)}
	if err := w.handle(context.Background(), delivery); err == nil {
		t.Fatal("expected the mailer error to propagate")
	}
}
