package argossql

import (
	"context"
	"database/sql/driver"
	"errors"
)

// wrappedConn implements driver.Conn plus every optional interface listed in
// doc.go's design notes. Each is implemented unconditionally; internally
// they check whether the wrapped conn actually supports the corresponding
// interface and either call through or replicate database/sql's own
// documented fallback - never claiming support the underlying driver
// doesn't have.
type wrappedConn struct {
	conn driver.Conn
	cfg  *config
}

func (c *wrappedConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &wrappedStmt{stmt: stmt, cfg: c.cfg, query: query}, nil
}

func (c *wrappedConn) Close() error { return c.conn.Close() }

func (c *wrappedConn) Begin() (driver.Tx, error) {
	return c.conn.Begin() //nolint:staticcheck // required by driver.Conn
}

func (c *wrappedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if pc, ok := c.conn.(driver.ConnPrepareContext); ok {
		stmt, err := pc.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		return &wrappedStmt{stmt: stmt, cfg: c.cfg, query: query}, nil
	}
	// database/sql's own fallback when ConnPrepareContext is absent: call
	// the non-context Prepare. We replicate it rather than claim support we
	// don't have.
	return c.Prepare(query)
}

func (c *wrappedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	ctx, span, start := c.cfg.startSpan(ctx, "exec", query)
	result, err := execer.ExecContext(ctx, query, args)
	c.cfg.endSpan(ctx, span, "exec", start, err)
	return result, err
}

func (c *wrappedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	ctx, span, start := c.cfg.startSpan(ctx, "query", query)
	rows, err := queryer.QueryContext(ctx, query, args)
	c.cfg.endSpan(ctx, span, "query", start, err)
	return rows, err
}

var errIsolationUnsupported = errors.New("argossql: underlying driver does not implement driver.ConnBeginTx; custom isolation/read-only transactions are unsupported")

func (c *wrappedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if bc, ok := c.conn.(driver.ConnBeginTx); ok {
		return bc.BeginTx(ctx, opts)
	}
	if opts.Isolation != driver.IsolationLevel(0) || opts.ReadOnly {
		return nil, errIsolationUnsupported
	}
	return c.conn.Begin() //nolint:staticcheck // documented fallback for legacy drivers
}

func (c *wrappedConn) Ping(ctx context.Context) error {
	if p, ok := c.conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *wrappedConn) ResetSession(ctx context.Context) error {
	if r, ok := c.conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *wrappedConn) IsValid() bool {
	if v, ok := c.conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

func (c *wrappedConn) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := c.conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}
