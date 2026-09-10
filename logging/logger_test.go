package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

// memExporter is a minimal, synchronous in-memory sdklog.Exporter test
// double; the SDK has no official one at v0.22.0 (only the
// logtest.RecordFactory helper for building Records, not capturing them).
type memExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *memExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.records = append(e.records, records...)
	return nil
}
func (e *memExporter) Shutdown(context.Context) error   { return nil }
func (e *memExporter) ForceFlush(context.Context) error { return nil }

func (e *memExporter) all() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdklog.Record(nil), e.records...)
}

func newTestLogger(buf *bytes.Buffer) (Logger, *memExporter) {
	exp := &memExporter{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)))
	otelLogger := lp.Logger("test")
	return NewZerolog(buf, zerolog.DebugLevel, otelLogger), exp
}

func spanContext() (context.Context, trace.SpanContext) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	return trace.ContextWithSpanContext(context.Background(), sc), sc
}

func TestLogger_CorrelatesLocalAndOTel(t *testing.T) {
	var buf bytes.Buffer
	logger, exp := newTestLogger(&buf)
	ctx, sc := spanContext()

	logger.Info(ctx, "hello", F("k", "v"))

	var localLine map[string]any
	if err := json.Unmarshal(buf.Bytes(), &localLine); err != nil {
		t.Fatalf("local log line is not valid JSON: %v (%q)", err, buf.String())
	}
	if localLine["trace_id"] != sc.TraceID().String() {
		t.Errorf("local trace_id = %v, want %v", localLine["trace_id"], sc.TraceID().String())
	}
	if localLine["span_id"] != sc.SpanID().String() {
		t.Errorf("local span_id = %v, want %v", localLine["span_id"], sc.SpanID().String())
	}
	if localLine["k"] != "v" {
		t.Errorf("local field k = %v, want v", localLine["k"])
	}

	records := exp.all()
	if len(records) != 1 {
		t.Fatalf("expected 1 emitted otel log record, got %d", len(records))
	}
	got := records[0]
	if got.TraceID() != sc.TraceID() || got.SpanID() != sc.SpanID() {
		t.Errorf("otel record trace/span = %v/%v, want %v/%v", got.TraceID(), got.SpanID(), sc.TraceID(), sc.SpanID())
	}
	if got.Body().AsString() != "hello" {
		t.Errorf("otel record body = %q, want %q", got.Body().AsString(), "hello")
	}
}

func TestLogger_ErrorSetsErrOnRecord(t *testing.T) {
	var buf bytes.Buffer
	logger, exp := newTestLogger(&buf)
	ctx, _ := spanContext()

	wantErr := errors.New("boom")
	logger.Error(ctx, "failed", wantErr)

	records := exp.all()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].AttributesLen() == 0 {
		t.Error("expected derived exception attributes on the record")
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := newTestLogger(&buf)
	ctx, _ := spanContext()

	scoped := logger.With(F("component", "test"))
	scoped.Info(ctx, "hi")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if line["component"] != "test" {
		t.Errorf("component = %v, want test", line["component"])
	}
}

// BenchmarkLogger_Info documents the one accepted allocation on this
// interface: the variadic []KV slice built at each call site (see the
// Logger doc comment in logger.go).
func BenchmarkLogger_Info(b *testing.B) {
	logger, _ := newTestLogger(&bytes.Buffer{})
	ctx, _ := spanContext()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Info(ctx, "hello", F("k", "v"))
	}
}
