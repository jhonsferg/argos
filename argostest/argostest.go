// Package argostest gives argos integration tests (in this repo and in
// consumers') the two test fixtures every module's own tests otherwise
// hand-roll: an in-memory span exporter installed as the global
// TracerProvider, and a Logger that records calls instead of writing
// anywhere, installed as core/log's global default.
package argostest

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argoslog "github.com/jhonsferg/argos/log"
	"github.com/jhonsferg/argos/logging"
)

// NewTracer installs an in-memory SDK TracerProvider as the global
// TracerProvider for the duration of t, restoring whatever was previously
// installed on cleanup. Read spans back via the returned exporter's
// GetSpans().
func NewTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return exp
}

// Entry is one call recorded by a RecordingLogger.
type Entry struct {
	Level  string // "debug", "info", "warn", or "error"
	Msg    string
	Err    error
	Fields []logging.KV
}

// recordingState is shared by a RecordingLogger and every logger derived
// from it via With, so an entry recorded through a scoped child is still
// visible from the original handle's Entries().
type recordingState struct {
	mu      sync.Mutex
	entries []Entry
}

// RecordingLogger is a logging.Logger that stores every call for
// assertions instead of writing anywhere. Safe for concurrent use.
type RecordingLogger struct {
	state  *recordingState
	fields []logging.KV
}

func (l *RecordingLogger) record(level, msg string, err error, fields []logging.KV) {
	merged := make([]logging.KV, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	l.state.entries = append(l.state.entries, Entry{Level: level, Msg: msg, Err: err, Fields: merged})
}

func (l *RecordingLogger) Debug(_ context.Context, msg string, fields ...logging.KV) {
	l.record("debug", msg, nil, fields)
}

func (l *RecordingLogger) Info(_ context.Context, msg string, fields ...logging.KV) {
	l.record("info", msg, nil, fields)
}

func (l *RecordingLogger) Warn(_ context.Context, msg string, fields ...logging.KV) {
	l.record("warn", msg, nil, fields)
}

func (l *RecordingLogger) Error(_ context.Context, msg string, err error, fields ...logging.KV) {
	l.record("error", msg, err, fields)
}

// With returns a Logger that always includes fields on top of any passed
// per-call. Entries recorded through it still appear in the original
// handle's Entries(), since both share the same underlying state.
func (l *RecordingLogger) With(fields ...logging.KV) logging.Logger {
	merged := make([]logging.KV, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	return &RecordingLogger{state: l.state, fields: merged}
}

// Entries returns every call recorded so far, in order - including calls
// made through a Logger derived from this one via With.
func (l *RecordingLogger) Entries() []Entry {
	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	return append([]Entry(nil), l.state.entries...)
}

// NewLogger installs a *RecordingLogger as core/log's global default for
// the duration of t, restoring the no-op default on cleanup.
func NewLogger(t *testing.T) *RecordingLogger {
	t.Helper()
	rec := &RecordingLogger{state: &recordingState{}}
	argoslog.SetDefault(rec)
	t.Cleanup(func() { argoslog.SetDefault(nil) })
	return rec
}
