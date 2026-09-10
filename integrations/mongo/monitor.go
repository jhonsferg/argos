package argosmongo

import (
	"context"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/logging"
)

// NewMonitor builds an event.CommandMonitor recording a CLIENT span (and the
// shared db.client.operation.duration metric) per command. Started and
// Succeeded/Failed are separate driver callbacks correlated by RequestID -
// stashed in a sync.Map between them since there's no per-command struct the
// driver hands back to us the way there is with, say, GORM's Statement.
func NewMonitor(opts ...Option) *event.CommandMonitor {
	m := &monitor{cfg: newConfig(opts...)}
	return &event.CommandMonitor{
		Started:   m.started,
		Succeeded: m.succeeded,
		Failed:    m.failed,
	}
}

type monitor struct {
	cfg   *config
	spans sync.Map // int64 (RequestID) -> trace.Span
}

func (m *monitor) started(ctx context.Context, evt *event.CommandStartedEvent) {
	tracer := otel.Tracer(instrumentationName)
	_, span := tracer.Start(ctx, evt.CommandName, trace.WithSpanKind(trace.SpanKindClient))

	span.SetAttributes(
		semconv.DBSystemMongoDB,
		semconv.DBOperationName(evt.CommandName),
		semconv.DBNamespace(evt.DatabaseName),
	)
	switch {
	case m.cfg.queryTextMasked:
		span.SetAttributes(semconv.DBQueryText(maskCommand(evt.Command)))
	case m.cfg.queryText:
		span.SetAttributes(semconv.DBQueryText(evt.Command.String()))
	}

	m.spans.Store(evt.RequestID, span)
}

func (m *monitor) succeeded(ctx context.Context, evt *event.CommandSucceededEvent) {
	m.finish(ctx, evt.RequestID, evt.CommandFinishedEvent, nil)
}

func (m *monitor) failed(ctx context.Context, evt *event.CommandFailedEvent) {
	m.finish(ctx, evt.RequestID, evt.CommandFinishedEvent, evt.Failure)
}

func (m *monitor) finish(ctx context.Context, requestID int64, finished event.CommandFinishedEvent, err error) {
	v, ok := m.spans.LoadAndDelete(requestID)
	if !ok {
		return
	}
	span, ok := v.(trace.Span)
	if !ok {
		return
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if m.cfg.logger != nil {
			m.cfg.logger.Error(ctx, "mongo command failed", err, logging.F("command", finished.CommandName))
		}
	}
	span.End()

	if m.cfg.duration != nil {
		m.cfg.duration.Record(ctx, finished.Duration.Seconds(), metric.WithAttributes(
			semconv.DBSystemMongoDB,
			semconv.DBOperationName(finished.CommandName),
		))
	}
}

// maskCommand renders cmd keeping only its top-level field names (e.g.
// "{insert: ?, documents: ?, ordered: ?}") - every value, including nested
// documents, is redacted rather than recursively walked, the same
// "shape visible, values gone" tradeoff every other masked-capture mode in
// this repo makes for simplicity. Falls back to a fixed placeholder if the
// command can't be parsed as a document at all.
func maskCommand(cmd bson.Raw) string {
	elements, err := cmd.Elements()
	if err != nil {
		return "?"
	}
	keys := make([]string, len(elements))
	for i, el := range elements {
		keys[i] = el.Key() + ": ?"
	}
	return "{" + strings.Join(keys, ", ") + "}"
}
