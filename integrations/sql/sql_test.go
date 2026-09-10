package argossql_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/codes"

	core "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/argostest"
	argossql "github.com/jhonsferg/argos/integrations/sql"
)

func TestOpen_ModernDriver_ExecAndQuery(t *testing.T) {
	exp := argostest.NewTracer(t)

	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	rows, err := db.QueryContext(context.Background(), "SELECT * FROM t")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	_ = rows.Close()

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans (exec, query), got %d", len(spans))
	}
	if spans[0].Name != "exec" || spans[1].Name != "query" {
		t.Errorf("span names = %q, %q; want exec, query", spans[0].Name, spans[1].Name)
	}
}

func TestOpen_LegacyDriver_FallsBackThroughPrepare(t *testing.T) {
	exp := argostest.NewTracer(t)

	db, err := argossql.Open("argossql_fake_legacy", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// The legacy conn implements neither ExecerContext nor QueryerContext,
	// so this must flow through wrappedConn.ExecContext -> ErrSkip ->
	// database/sql's own Prepare+Exec fallback -> wrappedStmt.
	if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span from the Prepare+Exec fallback, got %d", len(spans))
	}
	if spans[0].Name != "exec" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "exec")
	}
}

func TestOpen_RecordsErrorOnFailure(t *testing.T) {
	exp := argostest.NewTracer(t)

	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), "INSERT FAIL"); err == nil {
		t.Fatal("expected an error from the forced-failure query")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}

func TestOpen_QueryTextOptIn(t *testing.T) {
	exp := argostest.NewTracer(t)

	db, err := argossql.Open("argossql_fake_modern", "dsn", argossql.WithQueryText(true))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	const query = "SELECT secret FROM accounts"
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	var found bool
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.query.text" && kv.Value.AsString() == query {
			found = true
		}
	}
	if !found {
		t.Error("expected db.query.text attribute when WithQueryText(true) is set")
	}
}

func TestOpen_MaskedQueryTextOptIn(t *testing.T) {
	exp := argostest.NewTracer(t)

	db, err := argossql.Open("argossql_fake_modern", "dsn", argossql.WithMaskedQueryText(true))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	const query = "SELECT secret FROM accounts WHERE id = 5 AND name = 'bob'"
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	var got string
	var found bool
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			got = kv.Value.AsString()
			found = true
		}
	}
	if !found {
		t.Fatal("expected db.query.text attribute when WithMaskedQueryText(true) is set")
	}
	if got == query {
		t.Error("db.query.text was captured raw, want masked (literals redacted)")
	}
	if strings.Contains(got, "5") || strings.Contains(got, "bob") {
		t.Errorf("db.query.text = %q, still contains literal values", got)
	}
}

func TestOpen_MaskedQueryText_TakesPrecedenceOverRaw(t *testing.T) {
	exp := argostest.NewTracer(t)

	db, err := argossql.Open("argossql_fake_modern", "dsn", argossql.WithQueryText(true), argossql.WithMaskedQueryText(true))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	const query = "SELECT secret FROM accounts WHERE id = 5"
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	spans := exp.GetSpans()
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.query.text" && strings.Contains(kv.Value.AsString(), "5") {
			t.Errorf("db.query.text = %q, want masked even with WithQueryText also enabled", kv.Value.AsString())
		}
	}
}

func TestOpen_WithYAMLConfig_QueryText(t *testing.T) {
	exp := argostest.NewTracer(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n  sql:\n    query_text: true\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := core.FromYAML(path)
	if err != nil {
		t.Fatalf("FromYAML: %v", err)
	}

	db, err := argossql.Open("argossql_fake_modern", "dsn", argossql.WithYAMLConfig(cfg))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	const query = "SELECT secret FROM accounts"
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	var found bool
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.query.text" && kv.Value.AsString() == query {
			found = true
		}
	}
	if !found {
		t.Error("expected db.query.text attribute when the YAML section sets query_text: true")
	}
}

func TestOpen_QueryTextOffByDefault(t *testing.T) {
	exp := argostest.NewTracer(t)

	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), "SELECT secret FROM accounts"); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	spans := exp.GetSpans()
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.query.text" {
			t.Error("db.query.text must not be recorded unless WithQueryText(true) is set")
		}
	}
}
