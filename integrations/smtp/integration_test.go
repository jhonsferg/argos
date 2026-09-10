//go:build integration

package argossmtp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/mailpit"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argossmtp "github.com/jhonsferg/argos/integrations/smtp"
)

type mailpitMessagesResponse struct {
	Messages []struct {
		Subject string `json:"Subject"`
	} `json:"messages"`
}

// TestSendMail_RealServer proves the wrapper against an actual SMTP server,
// asserting the message it sends really arrives (via Mailpit's HTTP API),
// not just that the client-side call returned no error.
func TestSendMail_RealServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := mailpit.Run(ctx, "axllent/mailpit:v1.20")
	if err != nil {
		t.Fatalf("start mailpit container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	smtpAddr, err := container.SMTPEndpoint(ctx)
	if err != nil {
		t.Fatalf("SMTPEndpoint: %v", err)
	}
	httpURL, err := container.HTTPURL(ctx)
	if err != nil {
		t.Fatalf("HTTPURL: %v", err)
	}

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	const subject = "argossmtp integration test"
	msg := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: " + subject + "\r\n\r\nhello from argossmtp\r\n")

	err = argossmtp.SendMail(ctx, smtpAddr, nil, "sender@example.com", []string{"recipient@example.com"}, msg)
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "send" {
		t.Fatalf("expected 1 span named send, got %+v", spans)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpURL+"/api/v1/messages", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/messages: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body mailpitMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode messages response: %v", err)
	}

	var found bool
	for _, m := range body.Messages {
		if m.Subject == subject {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected mailpit to have received a message with subject %q, got %+v", subject, body.Messages)
	}
}
