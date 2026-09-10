package argoscassandra

import (
	"context"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/capture"
	"github.com/jhonsferg/argos/logging"
)

// Observer implements both gocql.QueryObserver and gocql.BatchObserver.
type Observer struct{ cfg *config }

// New builds an Observer. Assign it to both cluster.QueryObserver and
// cluster.BatchObserver to cover single queries and batches.
func New(opts ...Option) *Observer {
	return &Observer{cfg: newConfig(opts...)}
}

// ObserveQuery implements gocql.QueryObserver.
func (o *Observer) ObserveQuery(ctx context.Context, q gocql.ObservedQuery) {
	operation := queryOperation(q.Statement)
	span := o.startSpan(ctx, operation, q.Keyspace, q.Statement, q.Start)
	o.endSpan(ctx, span, operation, q.Start, q.End, q.Err)
}

// ObserveBatch implements gocql.BatchObserver.
func (o *Observer) ObserveBatch(ctx context.Context, b gocql.ObservedBatch) {
	span := o.startSpan(ctx, "batch", b.Keyspace, strings.Join(b.Statements, "; "), b.Start)
	span.SetAttributes(semconv.MessagingBatchMessageCount(len(b.Statements)))
	o.endSpan(ctx, span, "batch", b.Start, b.End, b.Err)
}

func (o *Observer) startSpan(ctx context.Context, operation, keyspace, statement string, start time.Time) trace.Span {
	tracer := otel.Tracer(instrumentationName)
	_, span := tracer.Start(ctx, operation, trace.WithSpanKind(trace.SpanKindClient), trace.WithTimestamp(start))

	span.SetAttributes(semconv.DBSystemCassandra, semconv.DBOperationName(operation))
	if keyspace != "" {
		span.SetAttributes(semconv.DBNamespace(keyspace))
	}
	if statement != "" {
		switch {
		case o.cfg.queryTextMasked:
			span.SetAttributes(semconv.DBQueryText(capture.MaskSQL(statement)))
		case o.cfg.queryText:
			span.SetAttributes(semconv.DBQueryText(statement))
		}
	}
	return span
}

func (o *Observer) endSpan(ctx context.Context, span trace.Span, operation string, start, end time.Time, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if o.cfg.logger != nil {
			o.cfg.logger.Error(ctx, "cassandra query failed", err, logging.F("operation", operation))
		}
	}
	span.End(trace.WithTimestamp(end))

	if o.cfg.duration != nil {
		o.cfg.duration.Record(ctx, end.Sub(start).Seconds(), metric.WithAttributes(semconv.DBSystemCassandra, semconv.DBOperationName(operation)))
	}
}

// queryOperation extracts the first token of a CQL statement (e.g.
// "SELECT", "INSERT") as the db.operation.name - a cheap prefix scan, no
// full parsing.
func queryOperation(statement string) string {
	statement = strings.TrimSpace(statement)
	if i := strings.IndexByte(statement, ' '); i != -1 {
		return strings.ToUpper(statement[:i])
	}
	return strings.ToUpper(statement)
}
