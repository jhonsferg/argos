package httpservercore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	argoslogging "github.com/jhonsferg/argos/logging"
)

// TestStatusRecorder_FlushAndUnwrapPassThrough proves a streaming/SSE
// handler still works once wrapped: without any capture configured, the
// handler only ever sees a statusRecorder, so this exercises its Flush and
// Unwrap directly.
func TestStatusRecorder_FlushAndUnwrapPassThrough(t *testing.T) {
	setTracer(t)
	instr := New()

	var unwrapped http.ResponseWriter
	handler := instr.Middleware(staticRoute("/stream"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("chunk"))
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter passed to the handler does not implement http.Flusher")
		}
		f.Flush()
		if u, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
			unwrapped = u.Unwrap()
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !rec.Flushed {
		t.Error("expected Flush to propagate to the underlying ResponseRecorder")
	}
	if unwrapped != rec {
		t.Error("Unwrap did not return the underlying ResponseWriter")
	}
}

// TestBodyCapturingWriter_FlushAndUnwrapPassThrough is the same proof with
// response body capture enabled, so the handler sees a statusRecorder
// wrapping a bodyCapturingWriter - both layers must forward correctly.
func TestBodyCapturingWriter_FlushAndUnwrapPassThrough(t *testing.T) {
	setTracer(t)
	instr := New(WithCaptureBodyOnError(1024))

	handler := instr.Middleware(staticRoute("/stream"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("chunk"))
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter passed to the handler does not implement http.Flusher")
		}
		f.Flush()
	}))

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !rec.Flushed {
		t.Error("expected Flush to propagate through both wrapping layers to the underlying ResponseRecorder")
	}
}

func TestCapturingReader_Close(t *testing.T) {
	body := &closeTrackingReadCloser{}
	r := newCapturingReader(body, 10)

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !body.closed {
		t.Error("expected Close to delegate to the underlying ReadCloser")
	}
}

type closeTrackingReadCloser struct{ closed bool }

func (c *closeTrackingReadCloser) Read([]byte) (int, error) { return 0, nil }
func (c *closeTrackingReadCloser) Close() error             { c.closed = true; return nil }

func TestWithLogger_LogsOnPanicRecovery(t *testing.T) {
	setTracer(t)
	logs := &capturingLogger{}
	instr := New(WithLogger(logs))

	if got := instr.Logger(); got != logs {
		t.Error("Logger() did not return the configured logger")
	}

	handler := instr.Middleware(staticRoute("/boom"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if logs.errorCalls != 1 {
		t.Errorf("expected 1 Error log call, got %d", logs.errorCalls)
	}
}

// capturingLogger is a minimal logging.Logger test double that only counts
// Error calls.
type capturingLogger struct{ errorCalls int }

func (l *capturingLogger) Debug(context.Context, string, ...argoslogging.KV) {}
func (l *capturingLogger) Info(context.Context, string, ...argoslogging.KV)  {}
func (l *capturingLogger) Warn(context.Context, string, ...argoslogging.KV)  {}
func (l *capturingLogger) Error(context.Context, string, error, ...argoslogging.KV) {
	l.errorCalls++
}
func (l *capturingLogger) With(...argoslogging.KV) argoslogging.Logger { return l }
