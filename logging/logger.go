// Package logging defines the SDK-agnostic Logger interface Argos and every
// integration depend on, plus the default zerolog-backed implementation.
// Nothing outside this package may import zerolog directly - swap
// implementations by satisfying Logger.
package logging

import "context"

// KV is a single structured logging field.
type KV struct {
	Key   string
	Value any
}

// F builds a KV field. Kept short since it's called at every log site.
func F(key string, value any) KV { return KV{Key: key, Value: value} }

// Logger is the minimal structured, context-aware logging interface used
// throughout Argos. Implementations must read trace_id/span_id off ctx (via
// trace.SpanContextFromContext) and attach them automatically - callers
// never do this themselves.
//
// The variadic []KV parameter is the one accepted allocation on this
// hot path (see zerolog.go); everything else in the call is allocation-free
// on a warmed-up logger. This is deliberate: a zero-alloc variadic field API
// would require a builder-style call shape disproportionate to what F0
// needs, and the cost is measured by BenchmarkLogger_Info (see
// logger_test.go) rather than hidden.
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...KV)
	Info(ctx context.Context, msg string, fields ...KV)
	Warn(ctx context.Context, msg string, fields ...KV)
	Error(ctx context.Context, msg string, err error, fields ...KV)
	// With returns a Logger that always includes fields on top of any
	// passed per-call.
	With(fields ...KV) Logger
}
