package argosgorm_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	core "github.com/jhonsferg/argos"
	argosgorm "github.com/jhonsferg/argos/integrations/gorm"
	argoslogging "github.com/jhonsferg/argos/logging"
)

func TestWithSystem(t *testing.T) {
	exp := setTracer(t)
	db := openTestDB(t, argosgorm.WithSystem(semconv.DBSystemSqlite))
	exp.Reset()

	if err := db.Create(&item{Name: "widget"}).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "db.system" && kv.Value.AsString() == "sqlite" {
			found = true
		}
	}
	if !found {
		t.Error("expected db.system attribute when WithSystem is set")
	}
}

func TestWithLogger_LogsOnFailure(t *testing.T) {
	setTracer(t)
	logs := &capturingLogger{}
	db := openTestDB(t, argosgorm.WithLogger(logs))

	if err := db.Exec("NOT VALID SQL").Error; err == nil {
		t.Fatal("expected a syntax error")
	}
	if logs.errorCalls != 1 {
		t.Errorf("expected 1 Error log call, got %d", logs.errorCalls)
	}
}

func TestWithYAMLConfig_QueryText(t *testing.T) {
	exp := setTracer(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n  gorm:\n    query_text: true\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := core.FromYAML(path)
	if err != nil {
		t.Fatalf("FromYAML: %v", err)
	}

	db := openTestDB(t, argosgorm.WithYAMLConfig(cfg))
	exp.Reset()

	if err := db.Create(&item{Name: "widget"}).Error; err != nil {
		t.Fatalf("Create: %v", err)
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
