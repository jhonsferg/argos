package httpservercore

import (
	"bytes"
	"io"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/capture"
)

// WithCaptureBodyOnError enables capturing up to maxBytes of the request
// and response bodies, attached as span attributes ("argos.http.request.body"
// / "argos.http.response.body") only when the request ends in a 5xx status -
// never on success. Off by default (maxBytes <= 0): request/response bodies
// can carry PII or secrets, so this is an explicit opt-in with a bounded
// size, not a default-on attribute, matching every other sensitive
// attribute in this repo. Shorthand for WithCapture with both body rules
// set to {Enabled: true, MaxBytes: maxBytes, OnErrorOnly: true} - use
// WithCapture directly for header capture or always-on body capture.
func WithCaptureBodyOnError(maxBytes int) Option {
	return func(i *Instrumentor) {
		rule := capture.Rule{Enabled: maxBytes > 0, MaxBytes: maxBytes, OnErrorOnly: true}
		i.rules.RequestBody = rule
		i.rules.ResponseBody = rule
	}
}

// WithCapture configures full request/response body and header capture per
// rules - see capture.HTTPRules. Replaces whatever rules an earlier Option
// (e.g. WithCaptureBodyOnError) set, so apply it first if combining both.
func WithCapture(rules capture.HTTPRules) Option {
	return func(i *Instrumentor) { i.rules = rules }
}

// capturingReader tees up to max bytes of what's read from next into an
// internal buffer, without altering what the caller observes - Read always
// returns next's own bytes/error unchanged, so wrapping a request body this
// way is transparent to the handler.
type capturingReader struct {
	next io.ReadCloser
	buf  bytes.Buffer
	max  int
}

func newCapturingReader(next io.ReadCloser, max int) *capturingReader {
	return &capturingReader{next: next, max: max}
}

func (c *capturingReader) Read(p []byte) (int, error) {
	n, err := c.next.Read(p)
	if n > 0 {
		c.tee(p[:n])
	}
	return n, err
}

func (c *capturingReader) Close() error { return c.next.Close() }

func (c *capturingReader) tee(b []byte) {
	remaining := c.max - c.buf.Len()
	if remaining <= 0 {
		return
	}
	if remaining < len(b) {
		b = b[:remaining]
	}
	c.buf.Write(b)
}

// bodyCapturingWriter sits below statusRecorder (statusRecorder wraps this,
// not the other way around) so status/header tracking is unaffected; it
// only tees what's written into a bounded buffer and passes every call
// through unchanged.
type bodyCapturingWriter struct {
	http.ResponseWriter
	buf bytes.Buffer
	max int
}

func newBodyCapturingWriter(w http.ResponseWriter, max int) *bodyCapturingWriter {
	return &bodyCapturingWriter{ResponseWriter: w, max: max}
}

func (w *bodyCapturingWriter) Write(b []byte) (int, error) {
	remaining := w.max - w.buf.Len()
	if remaining > 0 {
		captured := b
		if remaining < len(captured) {
			captured = captured[:remaining]
		}
		w.buf.Write(captured)
	}
	return w.ResponseWriter.Write(b)
}

// Flush passes through to the underlying ResponseWriter's http.Flusher, if
// it implements one - same reason statusRecorder does this.
func (w *bodyCapturingWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the underlying ResponseWriter, so a statusRecorder wrapped
// around this (and, through it, http.ResponseController) can still reach
// features (e.g. http.Hijacker) implemented further down the chain.
func (w *bodyCapturingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// attachCapturedRequestBody attaches the captured request body to span, if
// anything was actually captured. Callers only invoke this once they've
// already decided capture applies, per the configured Rule.
func attachCapturedRequestBody(span trace.Span, req *capturingReader) {
	if req != nil && req.buf.Len() > 0 {
		span.SetAttributes(attribute.String("argos.http.request.body", req.buf.String()))
	}
}

// attachCapturedResponseBody is attachCapturedRequestBody for the response
// side.
func attachCapturedResponseBody(span trace.Span, resp *bodyCapturingWriter) {
	if resp != nil && resp.buf.Len() > 0 {
		span.SetAttributes(attribute.String("argos.http.response.body", resp.buf.String()))
	}
}
