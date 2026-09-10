package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
)

func TestLogger_Levels(t *testing.T) {
	cases := []struct {
		name      string
		call      func(l Logger, ctx context.Context, msg string)
		wantLevel string
		wantSev   otellog.Severity
	}{
		{"debug", func(l Logger, ctx context.Context, msg string) { l.Debug(ctx, msg) }, "debug", otellog.SeverityDebug},
		{"warn", func(l Logger, ctx context.Context, msg string) { l.Warn(ctx, msg) }, "warn", otellog.SeverityWarn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger, exp := newTestLogger(&buf)
			ctx, sc := spanContext()

			tc.call(logger, ctx, "hi")

			var line map[string]any
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("invalid JSON: %v (%q)", err, buf.String())
			}
			if line["level"] != tc.wantLevel {
				t.Errorf("level = %v, want %v", line["level"], tc.wantLevel)
			}
			if line["trace_id"] != sc.TraceID().String() {
				t.Errorf("trace_id = %v, want %v", line["trace_id"], sc.TraceID().String())
			}

			records := exp.all()
			if len(records) != 1 {
				t.Fatalf("expected 1 record, got %d", len(records))
			}
			if records[0].Severity() != tc.wantSev {
				t.Errorf("severity = %v, want %v", records[0].Severity(), tc.wantSev)
			}
		})
	}
}

func TestNewZerolog_NilOTelLoggerDefaultsToNoop(t *testing.T) {
	var buf bytes.Buffer
	logger := NewZerolog(&buf, zerolog.InfoLevel, nil)

	// Must not panic even though no otellog.Logger was supplied - the
	// noop.Logger{} fallback silently drops the OTel side, local logging
	// still works.
	logger.Info(context.Background(), "hello")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("invalid JSON: %v (%q)", err, buf.String())
	}
	if line["message"] != "hello" {
		t.Errorf("message = %v, want hello", line["message"])
	}
}

type stringerValue struct{ s string }

func (v stringerValue) String() string { return v.s }

func TestFieldTypes_LocalAndOTel(t *testing.T) {
	boomErr := errors.New("boom")

	cases := []struct {
		name       string
		value      any
		wantLocal  any
		wantOTelFn func(t *testing.T, v attribute.Value)
	}{
		{"string", "hello", "hello", func(t *testing.T, v attribute.Value) {
			if v.AsString() != "hello" {
				t.Errorf("otel value = %q, want %q", v.AsString(), "hello")
			}
		}},
		{"bool", true, true, func(t *testing.T, v attribute.Value) {
			if !v.AsBool() {
				t.Error("otel value = false, want true")
			}
		}},
		{"int", 42, float64(42), func(t *testing.T, v attribute.Value) {
			if v.AsInt64() != 42 {
				t.Errorf("otel value = %d, want 42", v.AsInt64())
			}
		}},
		{"int64", int64(43), float64(43), func(t *testing.T, v attribute.Value) {
			if v.AsInt64() != 43 {
				t.Errorf("otel value = %d, want 43", v.AsInt64())
			}
		}},
		{"float64", 3.5, 3.5, func(t *testing.T, v attribute.Value) {
			if v.AsFloat64() != 3.5 {
				t.Errorf("otel value = %v, want 3.5", v.AsFloat64())
			}
		}},
		{"error", boomErr, "boom", func(t *testing.T, v attribute.Value) {
			if v.AsString() != "boom" {
				t.Errorf("otel value = %q, want %q", v.AsString(), "boom")
			}
		}},
		{"stringer", stringerValue{"stringed"}, "stringed", func(t *testing.T, v attribute.Value) {
			if v.AsString() != "stringed" {
				t.Errorf("otel value = %q, want %q", v.AsString(), "stringed")
			}
		}},
		{"default fallback", []int{1, 2, 3}, []any{float64(1), float64(2), float64(3)}, func(t *testing.T, v attribute.Value) {
			if v.AsString() != "[1 2 3]" {
				t.Errorf("otel value = %q, want %q", v.AsString(), "[1 2 3]")
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger, exp := newTestLogger(&buf)
			ctx, _ := spanContext()

			logger.Info(ctx, "msg", F("field", tc.value))

			var line map[string]any
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("invalid JSON: %v (%q)", err, buf.String())
			}
			got := line["field"]
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tc.wantLocal)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("local field = %s, want %s", gotJSON, wantJSON)
			}

			records := exp.all()
			if len(records) != 1 {
				t.Fatalf("expected 1 record, got %d", len(records))
			}
			var found bool
			records[0].WalkAttributes(func(kv attribute.KeyValue) bool {
				if kv.Key == "field" {
					found = true
					tc.wantOTelFn(t, kv.Value)
				}
				return true
			})
			if !found {
				t.Error("no \"field\" attribute found on the otel record")
			}
		})
	}
}
