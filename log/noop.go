package log

import (
	"context"

	"github.com/jhonsferg/argos/logging"
)

// noopLogger discards every call. It is the default before argos.Init runs
// so package-level calls never nil-panic (e.g. from a package init() or a
// test that never bootstraps argos).
type noopLogger struct{}

func (noopLogger) Debug(context.Context, string, ...logging.KV)        {}
func (noopLogger) Info(context.Context, string, ...logging.KV)         {}
func (noopLogger) Warn(context.Context, string, ...logging.KV)         {}
func (noopLogger) Error(context.Context, string, error, ...logging.KV) {}
func (n noopLogger) With(...logging.KV) logging.Logger                 { return n }
