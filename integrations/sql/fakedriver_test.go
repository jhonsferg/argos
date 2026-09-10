package argossql_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
)

// fakeDriver backs the unit tests with two conn flavors - legacyConn
// implements only the required driver.Conn/driver.Stmt methods, modernConn
// additionally implements every context-aware optional interface - so both
// of argossql's code paths (fast path vs database/sql's own fallback) are
// exercised without needing a real database. A query containing "FAIL"
// makes the fake return an error, for testing the error-recording path.
type fakeDriver struct{ modern bool }

func (d *fakeDriver) Open(string) (driver.Conn, error) {
	if d.modern {
		return &modernConn{}, nil
	}
	return &legacyConn{}, nil
}

func init() {
	sql.Register("argossql_fake_legacy", &fakeDriver{modern: false})
	sql.Register("argossql_fake_modern", &fakeDriver{modern: true})
	sql.Register("argossql_fake_connector", &connectorDriver{})
}

var errFake = errors.New("fake driver: forced failure")

func shouldFail(query string) error {
	if strings.Contains(query, "FAIL") {
		return errFake
	}
	return nil
}

// --- legacy-only conn/stmt ---

type legacyConn struct{}

func (c *legacyConn) Prepare(query string) (driver.Stmt, error) {
	if query == "FAIL_PREPARE" {
		return nil, errFake
	}
	return &legacyStmt{query: query}, nil
}
func (c *legacyConn) Close() error              { return nil }
func (c *legacyConn) Begin() (driver.Tx, error) { return fakeTx{}, nil } //nolint:staticcheck

type legacyStmt struct{ query string }

func (s *legacyStmt) Close() error  { return nil }
func (s *legacyStmt) NumInput() int { return -1 }
func (s *legacyStmt) Exec([]driver.Value) (driver.Result, error) { //nolint:staticcheck
	if err := shouldFail(s.query); err != nil {
		return nil, err
	}
	return fakeResult{}, nil
}

func (s *legacyStmt) Query([]driver.Value) (driver.Rows, error) { //nolint:staticcheck
	if err := shouldFail(s.query); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

// --- modern conn/stmt: every optional interface implemented ---

type modernConn struct{}

func (c *modernConn) Prepare(query string) (driver.Stmt, error) {
	if query == "FAIL_PREPARE" {
		return nil, errFake
	}
	return &modernStmt{query: query}, nil
}
func (c *modernConn) Close() error              { return nil }
func (c *modernConn) Begin() (driver.Tx, error) { return fakeTx{}, nil } //nolint:staticcheck

func (c *modernConn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	if query == "FAIL_PREPARE" {
		return nil, errFake
	}
	return &modernStmt{query: query}, nil
}

// CheckNamedValue rejects negative ints - just enough of a real check to
// prove the delegate path is reached and actually consulted, not a no-op.
func (c *modernConn) CheckNamedValue(nv *driver.NamedValue) error {
	if n, ok := nv.Value.(int64); ok && n < 0 {
		return errFake
	}
	return nil
}

func (c *modernConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if err := shouldFail(query); err != nil {
		return nil, err
	}
	return fakeResult{}, nil
}

func (c *modernConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if err := shouldFail(query); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

func (c *modernConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return fakeTx{}, nil
}
func (c *modernConn) Ping(context.Context) error         { return nil }
func (c *modernConn) ResetSession(context.Context) error { return nil }
func (c *modernConn) IsValid() bool                      { return true }

type modernStmt struct{ query string }

func (s *modernStmt) Close() error                               { return nil }
func (s *modernStmt) NumInput() int                              { return -1 }
func (s *modernStmt) Exec([]driver.Value) (driver.Result, error) { return fakeResult{}, nil } //nolint:staticcheck
func (s *modernStmt) Query([]driver.Value) (driver.Rows, error)  { return &fakeRows{}, nil }  //nolint:staticcheck

// CheckNamedValue mirrors modernConn's - see its comment.
func (s *modernStmt) CheckNamedValue(nv *driver.NamedValue) error {
	if n, ok := nv.Value.(int64); ok && n < 0 {
		return errFake
	}
	return nil
}

func (s *modernStmt) ExecContext(_ context.Context, _ []driver.NamedValue) (driver.Result, error) {
	if err := shouldFail(s.query); err != nil {
		return nil, err
	}
	return fakeResult{}, nil
}

func (s *modernStmt) QueryContext(_ context.Context, _ []driver.NamedValue) (driver.Rows, error) {
	if err := shouldFail(s.query); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

type fakeTx struct{}

func (fakeTx) Commit() error   { return nil }
func (fakeTx) Rollback() error { return nil }

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 1, nil }
func (fakeResult) RowsAffected() (int64, error) { return 1, nil }

type fakeRows struct{}

func (r *fakeRows) Columns() []string         { return []string{"id"} }
func (r *fakeRows) Close() error              { return nil }
func (r *fakeRows) Next([]driver.Value) error { return io.EOF }

// --- driver.DriverContext + driver.Connector fake, for the sql.OpenDB-style
// path (WrapConnector, wrappedConnector, and Open's OpenConnector branch) -
// none of the drivers above implement DriverContext, so that whole path was
// otherwise never exercised. Two DSNs trigger the two failure modes that
// path has to propagate un-wrapped: "FAIL_OPEN_CONNECTOR" fails during
// OpenConnector itself (at argossql.Open call time), "FAIL_CONNECT" opens
// fine but fails on the first real Connect (e.g. the first ping/query).
type connectorDriver struct{}

func (d *connectorDriver) Open(dsn string) (driver.Conn, error) { return &modernConn{}, nil }

func (d *connectorDriver) OpenConnector(dsn string) (driver.Connector, error) {
	if dsn == "FAIL_OPEN_CONNECTOR" {
		return nil, errFake
	}
	return &fakeConnector{dsn: dsn, driver: d}, nil
}

type fakeConnector struct {
	dsn    string
	driver driver.Driver
}

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) {
	if c.dsn == "FAIL_CONNECT" {
		return nil, errFake
	}
	return &modernConn{}, nil
}

func (c *fakeConnector) Driver() driver.Driver { return c.driver }
