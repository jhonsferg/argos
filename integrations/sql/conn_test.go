package argossql_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/jhonsferg/argos/argostest"
	argossql "github.com/jhonsferg/argos/integrations/sql"
	argoslogging "github.com/jhonsferg/argos/logging"
)

func TestConn_BeginTx_DelegatesWhenSupported(t *testing.T) {
	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
}

func TestConn_BeginTx_LegacyFallsBackToBeginForDefaultOptions(t *testing.T) {
	db, err := argossql.Open("argossql_fake_legacy", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx with default options should fall back to Begin, got: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
}

func TestConn_BeginTx_LegacyRejectsCustomIsolation(t *testing.T) {
	db, err := argossql.Open("argossql_fake_legacy", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err == nil {
		t.Fatal("expected an error requesting a read-only tx against a driver without ConnBeginTx")
	}
}

func TestConn_Ping(t *testing.T) {
	for _, driverName := range []string{"argossql_fake_modern", "argossql_fake_legacy"} {
		t.Run(driverName, func(t *testing.T) {
			db, err := argossql.Open(driverName, "dsn")
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer func() { _ = db.Close() }()

			if err := db.PingContext(context.Background()); err != nil {
				t.Errorf("PingContext: %v", err)
			}
		})
	}
}

func TestConn_QueryContext_Legacy(t *testing.T) {
	db, err := argossql.Open("argossql_fake_legacy", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(context.Background(), "SELECT * FROM t")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	_ = rows.Close()
}

func TestConn_CheckNamedValue_ConnPath(t *testing.T) {
	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (?)", 42); err != nil {
		t.Fatalf("ExecContext with an arg: %v", err)
	}
}

func TestConn_CheckNamedValue_StmtPath(t *testing.T) {
	db, err := argossql.Open("argossql_fake_legacy", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// The legacy conn has no ExecerContext, so this flows through the
	// Prepare+Exec fallback - wrappedStmt.CheckNamedValue, not
	// wrappedConn.CheckNamedValue.
	if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (?)", 42); err != nil {
		t.Fatalf("ExecContext with an arg: %v", err)
	}
}

func TestConn_NamedParameters_UnsupportedByLegacyDriver(t *testing.T) {
	db, err := argossql.Open("argossql_fake_legacy", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.ExecContext(context.Background(), "INSERT INTO t VALUES (:id)", sql.Named("id", 42))
	if err == nil {
		t.Fatal("expected an error: legacy driver's Stmt has no StmtExecContext, and named params can't convert to positional")
	}
}

func TestWrapConnector_HappyPath(t *testing.T) {
	db, err := argossql.Open("argossql_fake_connector", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	if d := db.Driver(); d == nil {
		t.Error("Driver() returned nil")
	}
}

func TestWrapConnector_OpenConnectorError(t *testing.T) {
	_, err := argossql.Open("argossql_fake_connector", "FAIL_OPEN_CONNECTOR")
	if err == nil {
		t.Fatal("expected the OpenConnector error to propagate from Open")
	}
}

func TestWrapConnector_ConnectError(t *testing.T) {
	db, err := argossql.Open("argossql_fake_connector", "FAIL_CONNECT")
	if err != nil {
		t.Fatalf("Open should succeed - OpenConnector itself doesn't fail: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(context.Background()); err == nil {
		t.Fatal("expected the underlying Connect error to propagate")
	}
}

func TestWithSystem(t *testing.T) {
	exp := argostest.NewTracer(t)

	db, err := argossql.Open("argossql_fake_modern", "dsn", argossql.WithSystem(attribute.String("db.system", "postgresql")))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	spans := exp.GetSpans()
	var found bool
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "db.system" && kv.Value.AsString() == "postgresql" {
			found = true
		}
	}
	if !found {
		t.Error("expected db.system attribute when WithSystem is set")
	}
}

func TestWithLogger_LogsOnFailure(t *testing.T) {
	logs := &capturingLogger{}
	db, err := argossql.Open("argossql_fake_modern", "dsn", argossql.WithLogger(logs))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), "INSERT FAIL"); err == nil {
		t.Fatal("expected an error from the forced-failure query")
	}
	if logs.errorCalls != 1 {
		t.Errorf("expected 1 Error log call, got %d", logs.errorCalls)
	}
}

func TestConn_PrepareContext_DelegatesWhenSupported(t *testing.T) {
	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	stmt, err := db.PrepareContext(context.Background(), "SELECT * FROM t")
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}
	defer func() { _ = stmt.Close() }()

	if _, err := stmt.ExecContext(context.Background()); err != nil {
		t.Errorf("prepared stmt ExecContext: %v", err)
	}
	rows, err := stmt.QueryContext(context.Background())
	if err != nil {
		t.Fatalf("prepared stmt QueryContext: %v", err)
	}
	_ = rows.Close()
}

func TestConn_PrepareContext_PropagatesUnderlyingError(t *testing.T) {
	for _, driverName := range []string{"argossql_fake_modern", "argossql_fake_legacy"} {
		t.Run(driverName, func(t *testing.T) {
			db, err := argossql.Open(driverName, "dsn")
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer func() { _ = db.Close() }()

			if _, err := db.PrepareContext(context.Background(), "FAIL_PREPARE"); err == nil {
				t.Fatal("expected the underlying Prepare/PrepareContext error to propagate")
			}
		})
	}
}

func TestConn_QueryContext_NamedParameters_UnsupportedByLegacyDriver(t *testing.T) {
	db, err := argossql.Open("argossql_fake_legacy", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.QueryContext(context.Background(), "SELECT * FROM t WHERE id = :id", sql.Named("id", 42))
	if err == nil {
		t.Fatal("expected an error: legacy driver's Stmt has no StmtQueryContext, and named params can't convert to positional")
	}
}

func TestConn_CheckNamedValue_RejectsViaConnDelegate(t *testing.T) {
	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.ExecContext(context.Background(), "INSERT INTO t VALUES (?)", int64(-1))
	if err == nil {
		t.Fatal("expected CheckNamedValue's delegate to reject a negative value")
	}
}

func TestConn_CheckNamedValue_RejectsViaStmtDelegate(t *testing.T) {
	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// A prepared statement's own CheckNamedValue (modernStmt implements it)
	// takes precedence over the conn's - go through Prepare explicitly so
	// this exercises wrappedStmt.CheckNamedValue, not wrappedConn's.
	stmt, err := db.PrepareContext(context.Background(), "INSERT INTO t VALUES (?)")
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}
	defer func() { _ = stmt.Close() }()

	if _, err := stmt.ExecContext(context.Background(), int64(-1)); err == nil {
		t.Fatal("expected the prepared statement's CheckNamedValue delegate to reject a negative value")
	}
}

func TestConn_ResetSession_LegacyFallbackOnPoolReuse(t *testing.T) {
	db, err := argossql.Open("argossql_fake_legacy", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1) // force the second call to reuse the same pooled conn

	for i := 0; i < 2; i++ {
		if _, err := db.ExecContext(context.Background(), "INSERT INTO t VALUES (1)"); err != nil {
			t.Fatalf("ExecContext #%d: %v", i, err)
		}
	}
}

func TestConn_Driver_PlainDSNPath(t *testing.T) {
	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if d := db.Driver(); d == nil {
		t.Error("Driver() returned nil")
	}
}

func TestConn_Begin_DirectInterfaceCall(t *testing.T) {
	db, err := argossql.Open("argossql_fake_modern", "dsn")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("Conn: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// database/sql itself never calls the plain (non-context) Begin on a
	// conn that also implements ConnBeginTx (which wrappedConn always
	// does) - it's only reachable by a caller going around database/sql
	// via Raw, the same way some non-standard tools do.
	err = conn.Raw(func(dc any) error {
		_, err := dc.(driver.Conn).Begin() //nolint:staticcheck // exercising the required interface method directly
		return err
	})
	if err != nil {
		t.Errorf("Begin via Raw: %v", err)
	}
}

// capturingLogger is a minimal logging.Logger test double that only counts
// Error calls - enough to verify WithLogger actually gets invoked on
// failure without pulling in a full logging fixture.
type capturingLogger struct{ errorCalls int }

func (l *capturingLogger) Debug(context.Context, string, ...argoslogging.KV) {}
func (l *capturingLogger) Info(context.Context, string, ...argoslogging.KV)  {}
func (l *capturingLogger) Warn(context.Context, string, ...argoslogging.KV)  {}
func (l *capturingLogger) Error(context.Context, string, error, ...argoslogging.KV) {
	l.errorCalls++
}
func (l *capturingLogger) With(...argoslogging.KV) argoslogging.Logger { return l }
