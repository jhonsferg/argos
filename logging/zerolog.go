package logging

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/trace"

	"github.com/jhonsferg/argos/internal/pool"
)

// zerologLogger is the default Logger implementation: it writes structured
// JSON locally via zerolog and, when an otellog.LoggerProvider is
// configured, emits the same record as an OTel log record over OTLP. Both
// paths derive trace_id/span_id from ctx - see otelbridge.go for how the
// OTLP side gets it for free from the SDK, and addTraceFields below for the
// local zerolog side.
type zerologLogger struct {
	zl     zerolog.Logger
	otel   otellog.Logger // never nil; defaults to noop.Logger{}
	fields []KV
}

// NewZerolog builds the default Logger. otelLogger may be nil, in which case
// OTLP log export is disabled and only local structured logs are written.
func NewZerolog(w io.Writer, level zerolog.Level, otelLogger otellog.Logger) Logger {
	if otelLogger == nil {
		otelLogger = noop.Logger{}
	}
	return &zerologLogger{
		zl:   zerolog.New(w).Level(level).With().Timestamp().Logger(),
		otel: otelLogger,
	}
}

func (l *zerologLogger) Debug(ctx context.Context, msg string, fields ...KV) {
	l.log(ctx, zerolog.DebugLevel, otellog.SeverityDebug, msg, nil, fields)
}

func (l *zerologLogger) Info(ctx context.Context, msg string, fields ...KV) {
	l.log(ctx, zerolog.InfoLevel, otellog.SeverityInfo, msg, nil, fields)
}

func (l *zerologLogger) Warn(ctx context.Context, msg string, fields ...KV) {
	l.log(ctx, zerolog.WarnLevel, otellog.SeverityWarn, msg, nil, fields)
}

func (l *zerologLogger) Error(ctx context.Context, msg string, err error, fields ...KV) {
	l.log(ctx, zerolog.ErrorLevel, otellog.SeverityError, msg, err, fields)
}

func (l *zerologLogger) With(fields ...KV) Logger {
	merged := make([]KV, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	return &zerologLogger{zl: l.zl, otel: l.otel, fields: merged}
}

func (l *zerologLogger) log(ctx context.Context, level zerolog.Level, sev otellog.Severity, msg string, err error, fields []KV) {
	ev := l.zl.WithLevel(level)
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		ev = ev.Str("trace_id", sc.TraceID().String()).Str("span_id", sc.SpanID().String())
	}
	if err != nil {
		ev = ev.Err(err)
	}
	for _, f := range l.fields {
		ev = addZerologField(ev, f)
	}
	for _, f := range fields {
		ev = addZerologField(ev, f)
	}
	ev.Msg(msg)

	l.emitOTel(ctx, sev, msg, err, fields)
}

func (l *zerologLogger) emitOTel(ctx context.Context, sev otellog.Severity, msg string, err error, fields []KV) {
	var rec otellog.Record
	rec.SetTimestamp(time.Now())
	rec.SetSeverity(sev)
	rec.SetBody(attribute.StringValue(msg))
	if err != nil {
		rec.SetErr(err)
	}

	attrs := pool.GetAttrs()
	for _, f := range l.fields {
		*attrs = append(*attrs, kvToAttribute(f))
	}
	for _, f := range fields {
		*attrs = append(*attrs, kvToAttribute(f))
	}
	rec.AddAttributes(*attrs...)
	pool.PutAttrs(attrs)

	// Trace/span correlation for this record is derived by the SDK itself
	// from ctx (see go.opentelemetry.io/otel/sdk/log logger.newRecord) -
	// nothing to set here.
	l.otel.Emit(ctx, rec)
}

func addZerologField(ev *zerolog.Event, f KV) *zerolog.Event {
	switch v := f.Value.(type) {
	case string:
		return ev.Str(f.Key, v)
	case bool:
		return ev.Bool(f.Key, v)
	case int:
		return ev.Int(f.Key, v)
	case int64:
		return ev.Int64(f.Key, v)
	case float64:
		return ev.Float64(f.Key, v)
	case error:
		return ev.AnErr(f.Key, v)
	case fmt.Stringer:
		return ev.Str(f.Key, v.String())
	default:
		return ev.Interface(f.Key, v)
	}
}

func kvToAttribute(f KV) attribute.KeyValue {
	switch v := f.Value.(type) {
	case string:
		return attribute.String(f.Key, v)
	case bool:
		return attribute.Bool(f.Key, v)
	case int:
		return attribute.Int(f.Key, v)
	case int64:
		return attribute.Int64(f.Key, v)
	case float64:
		return attribute.Float64(f.Key, v)
	case error:
		return attribute.String(f.Key, v.Error())
	case fmt.Stringer:
		return attribute.String(f.Key, v.String())
	default:
		return attribute.String(f.Key, fmt.Sprintf("%v", v))
	}
}
