package argosgorm

import (
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/jhonsferg/argos/capture"
	"github.com/jhonsferg/argos/logging"
)

// spanState is stashed on gorm.Statement.Settings between the Before and
// After callback for one operation - GORM gives each top-level call
// (db.Create, db.Find, ...) its own *Statement, so this never leaks across
// requests.
type spanState struct {
	span  trace.Span
	start time.Time
}

type spanStateKey struct{}

func (c *config) before(operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db.Statement.Context == nil {
			return
		}
		tracer := otel.Tracer(instrumentationName)
		ctx, span := tracer.Start(db.Statement.Context, operation, trace.WithSpanKind(trace.SpanKindClient))
		db.Statement.Context = ctx
		db.Statement.Settings.Store(spanStateKey{}, spanState{span: span, start: time.Now()})
	}
}

func (c *config) after(operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		raw, ok := db.Statement.Settings.Load(spanStateKey{})
		if !ok {
			return
		}
		state, ok := raw.(spanState)
		if !ok {
			return
		}

		metricAttrs := make([]attribute.KeyValue, 0, 2)
		if c.system.Key != "" {
			metricAttrs = append(metricAttrs, c.system)
		}
		metricAttrs = append(metricAttrs, semconv.DBOperationName(operation))

		spanAttrs := make([]attribute.KeyValue, len(metricAttrs), len(metricAttrs)+2)
		copy(spanAttrs, metricAttrs)
		if db.Statement.Table != "" {
			spanAttrs = append(spanAttrs, semconv.DBCollectionName(db.Statement.Table))
		}
		if sql := db.Statement.SQL.String(); sql != "" {
			switch {
			case c.queryTextMasked:
				spanAttrs = append(spanAttrs, semconv.DBQueryText(capture.MaskSQL(sql)))
			case c.queryText:
				spanAttrs = append(spanAttrs, semconv.DBQueryText(sql))
			}
		}
		state.span.SetAttributes(spanAttrs...)

		if db.Error != nil && db.Error != gorm.ErrRecordNotFound {
			state.span.RecordError(db.Error)
			state.span.SetStatus(codes.Error, db.Error.Error())
			if c.logger != nil {
				c.logger.Error(db.Statement.Context, "gorm operation failed", db.Error, logging.F("operation", operation))
			}
		}
		state.span.End()

		if c.duration != nil {
			c.duration.Record(db.Statement.Context, time.Since(state.start).Seconds(), metric.WithAttributes(metricAttrs...))
		}
	}
}
