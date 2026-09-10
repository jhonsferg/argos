package argossql

// Design notes on database/sql/driver coverage.
//
// database/sql resolves the fastest available path for every call: if a
// Conn implements ExecerContext/QueryerContext it's used directly; otherwise
// database/sql prepares a statement and calls Exec/Query on it. To
// instrument every query regardless of which path a given driver takes,
// wrappedConn (conn.go) and wrappedStmt (stmt.go) both start/end spans, and
// wrappedConn's ExecContext/QueryContext return driver.ErrSkip - the
// interfaces' own documented "fast path unavailable, use the fallback"
// signal - whenever the wrapped driver doesn't actually implement the fast
// path, so database/sql transparently falls through to the Prepare+Exec
// route this package also instruments. A caller never sees two spans for
// one call: exactly one of the two paths runs per query.
//
// PrepareContext/BeginTx/Ping/ResetSession/IsValid/CheckNamedValue are
// implemented unconditionally on the wrapper (rather than only when the
// wrapped driver has them, which Go's static interface satisfaction can't
// express per-value anyway) and each internally replicates database/sql's
// own documented fallback when the wrapped driver lacks the interface:
// PrepareContext falls back to Prepare, BeginTx falls back to the legacy
// Begin (erroring only if the caller actually requested a non-default
// isolation level or read-only transaction, which can't be honored without
// ConnBeginTx), Ping/ResetSession are no-ops, IsValid assumes true, and
// CheckNamedValue returns ErrSkip so database/sql applies its own default
// argument conversion. This is a deliberate simplification versus
// production libraries like otelsql, which generate a distinct wrapper
// struct per combination of supported optional interfaces to avoid ever
// claiming support that isn't there - this package instead claims support
// unconditionally and replicates the well-defined "unsupported" behavior
// dynamically. The externally observable behavior is identical either way;
// the trade-off is a handful of extra type assertions per call on drivers
// that don't support the modern interfaces; every actively maintained
// driver this instruments (pgx, mysql, sqlite3, sqlserver) does.
