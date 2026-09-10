// Package log gives every layer of an application ambient access to the
// Logger argos.Init configured, the same way otel.Tracer/otel.Meter give
// ambient access to tracing/metrics - no Logger instance needs to be
// threaded through constructors just so a repository or a background worker
// can emit a correlated log line.
//
// Import it under an alias if the file already imports the stdlib "log"
// package:
//
//	argoslog "github.com/jhonsferg/argos/log"
package log

import (
	"context"
	"sync/atomic"

	"github.com/jhonsferg/argos/logging"
)

// KV re-exports logging.KV; F re-exports logging.F - so a file importing
// only this package can still build structured fields for the calls below.
type KV = logging.KV

var F = logging.F

var global atomic.Pointer[logging.Logger]

func init() {
	var l logging.Logger = noopLogger{}
	global.Store(&l)
}

// SetDefault installs l as the Logger every package-level function in this
// package delegates to. argos.Init calls this automatically; call it
// directly only outside of Init (tests, or a Logger built without Init).
func SetDefault(l logging.Logger) {
	if l == nil {
		l = noopLogger{}
	}
	global.Store(&l)
}

// Default returns the currently installed Logger. Before argos.Init runs it
// is a no-op Logger, never nil.
func Default() logging.Logger { return *global.Load() }

// Debug logs at debug level through the currently installed Logger.
func Debug(ctx context.Context, msg string, fields ...logging.KV) {
	Default().Debug(ctx, msg, fields...)
}

// Info logs at info level through the currently installed Logger.
func Info(ctx context.Context, msg string, fields ...logging.KV) {
	Default().Info(ctx, msg, fields...)
}

// Warn logs at warn level through the currently installed Logger.
func Warn(ctx context.Context, msg string, fields ...logging.KV) {
	Default().Warn(ctx, msg, fields...)
}

// Error logs at error level through the currently installed Logger.
func Error(ctx context.Context, msg string, err error, fields ...logging.KV) {
	Default().Error(ctx, msg, err, fields...)
}

// With returns a Logger derived from the currently installed Logger that
// always includes fields on top of any passed per-call.
func With(fields ...logging.KV) logging.Logger { return Default().With(fields...) }
