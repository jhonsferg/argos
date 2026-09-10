package messagingcore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	messagingcore "github.com/jhonsferg/argos/integrations/messaging/core"
	argoslogging "github.com/jhonsferg/argos/logging"
)

func TestWithLogger_LogsOnFailure(t *testing.T) {
	setUp(t)
	logs := &capturingLogger{}
	instr := messagingcore.New(messagingcore.WithLogger(logs))

	ctx, span := instr.StartProducer(context.Background(), semconv.MessagingSystemRabbitmq, "q", propagation.MapCarrier{})
	instr.End(ctx, span, semconv.MessagingSystemRabbitmq, "publish", time.Now(), errors.New("broker unavailable"))

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
