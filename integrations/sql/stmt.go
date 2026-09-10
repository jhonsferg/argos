package argossql

import (
	"context"
	"database/sql/driver"
	"errors"
)

// wrappedStmt covers the prepare-then-execute path: whenever a Conn doesn't
// implement ExecerContext/QueryerContext (or plain database/sql code calls
// db.Prepare directly), execution flows through here instead, so it gets the
// same span treatment as the fast path in conn.go - the caller sees one
// consistent story regardless of which path the driver took.
type wrappedStmt struct {
	stmt  driver.Stmt
	cfg   *config
	query string
}

func (s *wrappedStmt) Close() error  { return s.stmt.Close() }
func (s *wrappedStmt) NumInput() int { return s.stmt.NumInput() }

func (s *wrappedStmt) Exec(args []driver.Value) (driver.Result, error) { //nolint:staticcheck // required by driver.Stmt
	ctx, span, start := s.cfg.startSpan(context.Background(), "exec", s.query)
	result, err := s.stmt.Exec(args) //nolint:staticcheck // legacy path, no context to forward
	s.cfg.endSpan(ctx, span, "exec", start, err)
	return result, err
}

func (s *wrappedStmt) Query(args []driver.Value) (driver.Rows, error) { //nolint:staticcheck // required by driver.Stmt
	ctx, span, start := s.cfg.startSpan(context.Background(), "query", s.query)
	rows, err := s.stmt.Query(args) //nolint:staticcheck // legacy path, no context to forward
	s.cfg.endSpan(ctx, span, "query", start, err)
	return rows, err
}

var errNamedParamsUnsupported = errors.New("argossql: underlying driver does not implement driver.StmtExecContext/StmtQueryContext; named parameters require a context-aware driver")

func (s *wrappedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if ec, ok := s.stmt.(driver.StmtExecContext); ok {
		ctx, span, start := s.cfg.startSpan(ctx, "exec", s.query)
		result, err := ec.ExecContext(ctx, args)
		s.cfg.endSpan(ctx, span, "exec", start, err)
		return result, err
	}
	values, err := positionalValues(args)
	if err != nil {
		return nil, err
	}
	return s.Exec(values) //nolint:staticcheck // documented fallback for legacy drivers
}

func (s *wrappedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if qc, ok := s.stmt.(driver.StmtQueryContext); ok {
		ctx, span, start := s.cfg.startSpan(ctx, "query", s.query)
		rows, err := qc.QueryContext(ctx, args)
		s.cfg.endSpan(ctx, span, "query", start, err)
		return rows, err
	}
	values, err := positionalValues(args)
	if err != nil {
		return nil, err
	}
	return s.Query(values) //nolint:staticcheck // documented fallback for legacy drivers
}

func (s *wrappedStmt) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := s.stmt.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// positionalValues converts context-style named arguments back to the
// legacy positional []driver.Value shape database/sql used before Go 1.8,
// which is what Stmt.Exec/Query still accept. It only succeeds for purely
// positional arguments (no Name set) - the same constraint database/sql
// itself applies when falling back for a driver lacking the context-aware
// interfaces.
func positionalValues(args []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		if arg.Name != "" {
			return nil, errNamedParamsUnsupported
		}
		values[i] = arg.Value
	}
	return values, nil
}
