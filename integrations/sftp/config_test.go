package argossftp_test

import (
	"context"
	"testing"

	argossftp "github.com/jhonsferg/argos/integrations/sftp"
	argoslogging "github.com/jhonsferg/argos/logging"
)

func TestWithLogger_LogsOnFailure(t *testing.T) {
	setTracer(t)
	logs := &capturingLogger{}
	client := argossftp.Wrap(newInMemoryClient(t), argossftp.WithLogger(logs))

	if _, err := client.OpenContext(context.Background(), "/does-not-exist.txt"); err == nil {
		t.Fatal("expected an error for a missing file")
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
