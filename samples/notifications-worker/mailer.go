package main

import (
	"context"

	argossmtp "github.com/jhonsferg/argos/integrations/smtp"
)

// Mailer sends a Notification as an email. *SMTPMailer is the real
// implementation; tests use a fake.
type Mailer interface {
	Send(ctx context.Context, n Notification) error
}

// SMTPMailer sends notifications via argossmtp.SendMail.
type SMTPMailer struct {
	addr string
	from string
}

func NewSMTPMailer(addr, from string) *SMTPMailer {
	return &SMTPMailer{addr: addr, from: from}
}

func (m *SMTPMailer) Send(ctx context.Context, n Notification) error {
	msg := []byte("From: " + m.from + "\r\n" +
		"To: " + n.To + "\r\n" +
		"Subject: " + n.Subject + "\r\n" +
		"\r\n" + n.Body + "\r\n")
	return argossmtp.SendMail(ctx, m.addr, nil, m.from, []string{n.To}, msg)
}
