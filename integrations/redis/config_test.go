package argosredis_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	core "github.com/jhonsferg/argos"
	argosredis "github.com/jhonsferg/argos/integrations/redis"
	argoslogging "github.com/jhonsferg/argos/logging"
)

func TestWithSystem_Override(t *testing.T) {
	exp := setTracer(t)
	client := newTestClient(t, argosredis.WithSystem(attribute.String("db.system", "valkey")))
	exp.Reset()
	ctx := context.Background()

	if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got string
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.system" {
			got = kv.Value.AsString()
		}
	}
	if got != "valkey" {
		t.Errorf("db.system = %q, want %q", got, "valkey")
	}
}

func TestWithLogger_LogsOnFailure(t *testing.T) {
	setTracer(t)
	logs := &capturingLogger{}
	client := newTestClient(t, argosredis.WithLogger(logs))
	ctx := context.Background()

	// A WRONGTYPE error, not redis.Nil - the hook must actually classify
	// this as a real failure to log it.
	if err := client.RPush(ctx, "a-list", "v").Err(); err != nil {
		t.Fatalf("RPush: %v", err)
	}
	if err := client.Get(ctx, "a-list").Err(); err == nil {
		t.Fatal("expected a WRONGTYPE error reading a list key as a string")
	}

	if logs.errorCalls == 0 {
		t.Error("expected at least 1 Error log call for the WRONGTYPE failure")
	}
}

func TestWithYAMLConfig_QueryText(t *testing.T) {
	exp := setTracer(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n  redis:\n    query_text: true\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := core.FromYAML(path)
	if err != nil {
		t.Fatalf("FromYAML: %v", err)
	}

	client := newTestClient(t, argosredis.WithYAMLConfig(cfg))
	exp.Reset()
	ctx := context.Background()

	if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

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
