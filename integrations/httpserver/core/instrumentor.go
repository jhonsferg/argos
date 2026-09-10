// Package httpservercore holds the framework-agnostic HTTP server
// instrumentation logic shared by every Argos router adapter (nethttp, chi,
// gin, ...). Route resolution, status capture, and span/metric bookkeeping
// are written once here; each adapter only translates its framework's
// request/response shape into a call into this package.
package httpservercore

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/capture"
	"github.com/jhonsferg/argos/logging"
	"github.com/jhonsferg/argos/middleware"
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/httpserver/core"

// RouteFunc resolves the matched route pattern (e.g. "/users/{id}") for r.
// It is called after the inner handler has run - see Middleware for why.
type RouteFunc func(r *http.Request) string

// Option configures an Instrumentor.
type Option func(*Instrumentor)

// WithLogger attaches a Logger so panics recovered by Middleware are also
// logged, not just recorded on the span. Omitting it still records/recovers
// panics - it just skips the log call (see core/middleware.Handle).
func WithLogger(l logging.Logger) Option {
	return func(i *Instrumentor) { i.logger = l }
}

// Instrumentor holds the (created-once) metric instrument and configuration
// shared across every request a Middleware built from it handles.
type Instrumentor struct {
	logger   logging.Logger
	duration metric.Float64Histogram
	rules    capture.HTTPRules
}

// New builds an Instrumentor. Create one per process (or per server) and
// reuse it - it owns the duration histogram, which must not be recreated
// per request.
func New(opts ...Option) *Instrumentor {
	i := &Instrumentor{}
	for _, opt := range opts {
		opt(i)
	}
	// Errors here mean a malformed instrument definition, not a runtime
	// failure; fall back to a no-op histogram rather than panicking.
	h, err := otel.Meter(instrumentationName).Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests"),
	)
	if err == nil {
		i.duration = h
	}
	return i
}

// Logger returns the configured Logger, or nil if none was set.
func (i *Instrumentor) Logger() logging.Logger { return i.logger }

// Start begins a server span for r. The span is named by method only -
// the route isn't known yet for most routers (see package doc on
// Middleware) - callers rename it via End once routeFunc has run.
//
// It first extracts any incoming W3C tracecontext/baggage from r's headers,
// so a server span started here becomes a child of whatever span (e.g. an
// argos-httpclient call) made the request, instead of an unlinked root span.
//
// Start is a thin wrapper around StartWithCarrier for net/http-shaped
// frameworks; frameworks not built on *http.Request (e.g. fasthttp-based
// ones) call StartWithCarrier directly with their own header carrier.
func (i *Instrumentor) Start(r *http.Request) (context.Context, trace.Span, time.Time) {
	return i.StartWithCarrier(r.Context(), r.Method, propagation.HeaderCarrier(r.Header))
}

// StartWithCarrier is Start generalized over any propagation.TextMapCarrier,
// for server frameworks that don't represent requests as *http.Request.
func (i *Instrumentor) StartWithCarrier(ctx context.Context, method string, carrier propagation.TextMapCarrier) (context.Context, trace.Span, time.Time) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindServer))
	return ctx, span, time.Now()
}

// End finalizes the span started by Start: renames it to "{method} {route}"
// (or just method if route is empty), sets the http.route/status attributes,
// marks the span an error for 5xx, records the duration histogram, and ends
// the span.
func (i *Instrumentor) End(ctx context.Context, span trace.Span, method, route string, status int, start time.Time) {
	name := method
	if route != "" {
		name = method + " " + route
	}
	span.SetName(name)

	attrs := [3]attribute.KeyValue{
		methodAttribute(method),
		semconv.HTTPResponseStatusCode(status),
		semconv.HTTPRoute(route),
	}
	span.SetAttributes(attrs[:]...)
	if status >= 500 {
		span.SetStatus(codes.Error, "")
	}
	span.End()

	if i.duration != nil {
		i.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs[0], attrs[1]))
	}
}

// Middleware wraps next with server instrumentation. routeFunc is called
// after next has run (on the *http.Request actually passed to it, not the
// caller's original - see package doc) so it works correctly with routers
// that only finish populating their route-match state during dispatch (the
// stdlib enhanced ServeMux's Request.Pattern and chi's RouteContext both
// behave this way).
func (i *Instrumentor) Middleware(routeFunc RouteFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span, start := i.Start(r)
			reqWithCtx := r.WithContext(ctx)

			var reqCapture *capturingReader
			var respCapture *bodyCapturingWriter
			baseWriter := w
			if i.rules.RequestBody.Enabled && reqWithCtx.Body != nil {
				reqCapture = newCapturingReader(reqWithCtx.Body, i.rules.RequestBody.MaxBytes)
				reqWithCtx.Body = reqCapture
			}
			if i.rules.ResponseBody.Enabled {
				respCapture = newBodyCapturingWriter(w, i.rules.ResponseBody.MaxBytes)
				baseWriter = respCapture
			}
			rw := newStatusRecorder(baseWriter)

			defer func() {
				// Recovering here - rather than re-panicking after handling
				// it - is deliberate: this middleware's whole purpose is to
				// stop the panic from reaching net/http's own connection-level
				// recovery, so the 500 written below actually reaches the
				// client instead of the connection being aborted.
				if err := middleware.Handle(ctx, i.logger, recover()); err != nil && !rw.wroteHeader {
					rw.WriteHeader(http.StatusInternalServerError)
				}
				route := routeFunc(reqWithCtx)
				isError := rw.status >= 500
				if i.rules.RequestBody.Enabled && (!i.rules.RequestBody.OnErrorOnly || isError) {
					attachCapturedRequestBody(span, reqCapture)
				}
				if i.rules.ResponseBody.Enabled && (!i.rules.ResponseBody.OnErrorOnly || isError) {
					attachCapturedResponseBody(span, respCapture)
				}
				capture.ApplyHeaderRule(span, "http.request.header.", reqWithCtx.Header, i.rules.RequestHeaders, isError)
				capture.ApplyHeaderRule(span, "http.response.header.", w.Header(), i.rules.ResponseHeaders, isError)
				i.End(ctx, span, reqWithCtx.Method, route, rw.status, start)
			}()

			next.ServeHTTP(rw, reqWithCtx)
		})
	}
}

// methodAttribute returns a precomputed KeyValue for known HTTP methods
// (avoiding a string allocation on the common path) and falls back to
// building one for anything else.
func methodAttribute(method string) attribute.KeyValue {
	switch method {
	case http.MethodGet:
		return semconv.HTTPRequestMethodGet
	case http.MethodPost:
		return semconv.HTTPRequestMethodPost
	case http.MethodPut:
		return semconv.HTTPRequestMethodPut
	case http.MethodPatch:
		return semconv.HTTPRequestMethodPatch
	case http.MethodDelete:
		return semconv.HTTPRequestMethodDelete
	case http.MethodHead:
		return semconv.HTTPRequestMethodHead
	case http.MethodOptions:
		return semconv.HTTPRequestMethodOptions
	default:
		return semconv.HTTPRequestMethodKey.String(method)
	}
}
