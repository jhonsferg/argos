package argosmongo_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"

	core "github.com/jhonsferg/argos"
	argosmongo "github.com/jhonsferg/argos/integrations/mongo"
	argoslogging "github.com/jhonsferg/argos/logging"
)

func TestWithLogger_LogsOnFailure(t *testing.T) {
	setTracer(t)
	logs := &capturingLogger{}
	mon := argosmongo.NewMonitor(argosmongo.WithLogger(logs))

	ctx := context.Background()
	mon.Started(ctx, &event.CommandStartedEvent{CommandName: "insert", DatabaseName: "orders", RequestID: 3})
	mon.Failed(ctx, &event.CommandFailedEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{CommandName: "insert", DatabaseName: "orders", RequestID: 3},
		Failure:              errors.New("duplicate key"),
	})

	if logs.errorCalls != 1 {
		t.Errorf("expected 1 Error log call, got %d", logs.errorCalls)
	}
}

func TestWithYAMLConfig_QueryText(t *testing.T) {
	exp := setTracer(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n  mongo:\n    query_text: true\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := core.FromYAML(path)
	if err != nil {
		t.Fatalf("FromYAML: %v", err)
	}

	mon := argosmongo.NewMonitor(argosmongo.WithYAMLConfig(cfg))

	cmd, err := bson.Marshal(bson.D{{Key: "find", Value: "orders"}})
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}

	ctx := context.Background()
	mon.Started(ctx, &event.CommandStartedEvent{
		CommandName:  "find",
		DatabaseName: "orders",
		RequestID:    4,
		Command:      bson.Raw(cmd),
	})
	mon.Succeeded(ctx, &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{CommandName: "find", DatabaseName: "orders", RequestID: 4},
	})

	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			found = true
		}
	}
	if !found {
		t.Error("expected db.query.text attribute when the YAML section sets query_text: true")
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
