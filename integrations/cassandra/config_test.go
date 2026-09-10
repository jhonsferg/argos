package argoscassandra_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gocql/gocql"

	core "github.com/jhonsferg/argos"
	argoscassandra "github.com/jhonsferg/argos/integrations/cassandra"
	argoslogging "github.com/jhonsferg/argos/logging"
)

func TestWithLogger_LogsOnFailure(t *testing.T) {
	setTracer(t)
	logs := &capturingLogger{}
	obs := argoscassandra.New(argoscassandra.WithLogger(logs))

	now := time.Now()
	obs.ObserveQuery(context.Background(), gocql.ObservedQuery{
		Statement: "INSERT INTO items (id) VALUES (?)",
		Start:     now,
		End:       now.Add(time.Millisecond),
		Err:       errors.New("timeout"),
	})

	if logs.errorCalls != 1 {
		t.Errorf("expected 1 Error log call, got %d", logs.errorCalls)
	}
}

func TestWithYAMLConfig_QueryText(t *testing.T) {
	exp := setTracer(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n  cassandra:\n    query_text: true\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := core.FromYAML(path)
	if err != nil {
		t.Fatalf("FromYAML: %v", err)
	}

	obs := argoscassandra.New(argoscassandra.WithYAMLConfig(cfg))

	start := time.Now()
	obs.ObserveQuery(context.Background(), gocql.ObservedQuery{
		Keyspace:  "orders",
		Statement: "SELECT * FROM items WHERE id = ?",
		Start:     start,
		End:       start.Add(5 * time.Millisecond),
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
