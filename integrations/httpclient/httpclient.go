// Package argoshttpclient instruments outgoing HTTP calls by wrapping an
// http.RoundTripper, the same shape used by the ecosystem's own
// otelhttp.NewTransport - a drop-in Transport, not a bespoke client API.
package argoshttpclient

import (
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
)

const instrumentationName = "github.com/jhonsferg/argos/integrations/httpclient"

// Option configures the RoundTripper built by Wrap/NewClient.
type Option func(*roundTripper)

// WithCaptureBodyOnError enables capturing up to maxBytes of the request
// and response bodies, attached as span attributes ("argos.http.request.body"
// / "argos.http.response.body") only when the call errors or the response
// status is >= 500 - never on success. Off by default (maxBytes <= 0), same
// PII/size rationale as httpservercore.WithCaptureBodyOnError. Shorthand for
// WithCapture with both body rules set to {Enabled: true, MaxBytes: maxBytes,
// OnErrorOnly: true} - use WithCapture directly for header capture or
// always-on body capture.
func WithCaptureBodyOnError(maxBytes int) Option {
	return func(rt *roundTripper) {
		rule := capture.Rule{Enabled: maxBytes > 0, MaxBytes: maxBytes, OnErrorOnly: true}
		rt.rules.RequestBody = rule
		rt.rules.ResponseBody = rule
	}
}

// WithCapture configures full request/response body and header capture per
// rules - see capture.HTTPRules. Request headers are read from the
// outgoing request (after trace-context injection); response headers from
// the received response. Replaces whatever rules an earlier Option (e.g.
// WithCaptureBodyOnError) set, so apply it first if combining both.
func WithCapture(rules capture.HTTPRules) Option {
	return func(rt *roundTripper) { rt.rules = rules }
}

type roundTripper struct {
	next     http.RoundTripper
	duration metric.Float64Histogram
	rules    capture.HTTPRules
}

// Wrap returns an http.RoundTripper that instruments next with a client span
// and duration metric per request, injecting the active trace context into
// outgoing request headers. next defaults to http.DefaultTransport if nil.
func Wrap(next http.RoundTripper, opts ...Option) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	rt := &roundTripper{next: next}
	for _, opt := range opts {
		opt(rt)
	}
	if h, err := otel.Meter(instrumentationName).Float64Histogram(
		"http.client.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of outgoing HTTP requests"),
	); err == nil {
		rt.duration = h
	}
	return rt
}

// NewClient returns a shallow copy of base (or a zero-value *http.Client if
// base is nil) with its Transport wrapped by Wrap.
func NewClient(base *http.Client, opts ...Option) *http.Client {
	var clone http.Client
	if base != nil {
		clone = *base
	}
	clone.Transport = Wrap(clone.Transport, opts...)
	return &clone
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	tracer := otel.Tracer(instrumentationName)
	ctx, span := tracer.Start(ctx, req.Method, trace.WithSpanKind(trace.SpanKindClient))
	start := time.Now()

	// req.Clone is the one unavoidable allocation here: header injection
	// requires a request whose Header map we can mutate without racing the
	// caller, and http.RoundTripper implementations must not mutate the
	// request they're given.
	outReq := req.Clone(ctx)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(outReq.Header))

	// The request body must be captured before sending - once rt.next
	// consumes it there's nothing left to read back, unlike the response
	// body below, which is still unread at this point. Whether it gets
	// attached to the span depends on rules.RequestBody.OnErrorOnly, known
	// only after the call - but draining can't wait that long.
	var reqCapture []byte
	if rt.rules.RequestBody.Enabled {
		reqCapture, outReq.Body = drainAndCapture(outReq.Body, rt.rules.RequestBody.MaxBytes)
	}

	resp, err := rt.next.RoundTrip(outReq)

	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	isError := err != nil || status >= 500
	attrs := [2]attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(req.Method),
		semconv.ServerAddress(req.URL.Hostname()),
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))
		if status >= 500 {
			span.SetStatus(codes.Error, "")
		}
	}

	if rt.rules.RequestBody.Enabled && (!rt.rules.RequestBody.OnErrorOnly || isError) && len(reqCapture) > 0 {
		span.SetAttributes(attribute.String("argos.http.request.body", string(reqCapture)))
	}
	capture.ApplyHeaderRule(span, "http.request.header.", outReq.Header, rt.rules.RequestHeaders, isError)
	if resp != nil {
		if rt.rules.ResponseBody.Enabled && (!rt.rules.ResponseBody.OnErrorOnly || isError) {
			var respCapture []byte
			respCapture, resp.Body = drainAndCapture(resp.Body, rt.rules.ResponseBody.MaxBytes)
			if len(respCapture) > 0 {
				span.SetAttributes(attribute.String("argos.http.response.body", string(respCapture)))
			}
		}
		capture.ApplyHeaderRule(span, "http.response.header.", resp.Header, rt.rules.ResponseHeaders, isError)
	}
	span.SetAttributes(attrs[:]...)
	span.End()

	if rt.duration != nil {
		rt.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs[:]...))
	}

	return resp, err
}
