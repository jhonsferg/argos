// Package middleware holds cross-cutting helpers shared by every future
// integration adapter - for now, panic recovery.
package middleware

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/logging"
)

// Handle normalizes a recovered panic value (as returned by the builtin
// recover()) into an error, records it as an exception on the span found in
// ctx (if any) and marks that span's status as an error, and logs it.
//
// Handle does not re-panic and does not write any HTTP response - it is
// generic enough for background goroutines as well as HTTP handlers. An
// HTTP adapter middleware should call recover(), pass the result here, then
// decide how to respond (e.g. write a 500) using the returned error.
//
// Handle is a no-op (returns nil) when recovered is nil, so it's safe to
// call unconditionally with the result of recover().
func Handle(ctx context.Context, logger logging.Logger, recovered any) error {
	if recovered == nil {
		return nil
	}

	err := fmt.Errorf("argos: recovered panic: %v", recovered)

	span := trace.SpanFromContext(ctx)
	span.RecordError(err, trace.WithStackTrace(true))
	span.SetStatus(codes.Error, err.Error())

	if logger != nil {
		logger.Error(ctx, "recovered from panic", err)
	}

	return err
}
