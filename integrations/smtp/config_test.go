package argossmtp_test

import (
	"context"
	"testing"

	argossmtp "github.com/jhonsferg/argos/integrations/smtp"
	argoslogging "github.com/jhonsferg/argos/logging"
)

func TestWithLogger_LogsOnFailure(t *testing.T) {
	setTracer(t)
	addr := startFakeSMTPServer(t, rejectedRecipientScript)
	logs := &capturingLogger{}

	err := argossmtp.SendMail(context.Background(), addr, nil, "a@example.com", []string{"b@example.com"}, []byte(testMessage), argossmtp.WithLogger(logs))
	if err == nil {
		t.Fatal("expected an error for a rejected recipient")
	}
	if logs.errorCalls != 1 {
		t.Errorf("expected 1 Error log call, got %d", logs.errorCalls)
	}
}

// capturingLogger is a minimal logging.Logger test double that only counts
// Error calls.
type capturingLogger struct{ errorCalls int }

func (l *capturingLogger) Debug(context.Context, string, ...argoslogging.KV) {}
func (l *capturingLogger) Info(context.Context, string, ...argoslogging.KV)  {}
func (l *capturingLogger) Warn(context.Context, string, ...argoslogging.KV)  {}
func (l *capturingLogger) Error(context.Context, string, error, ...argoslogging.KV) {
	l.errorCalls++
}
func (l *capturingLogger) With(...argoslogging.KV) argoslogging.Logger { return l }
