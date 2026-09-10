package capture

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMaskSQL(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "string and numeric literals",
			query: "SELECT * FROM t WHERE id = 5 AND name = 'bob'",
			want:  "SELECT * FROM t WHERE id = ? AND name = ?",
		},
		{
			name:  "no literals",
			query: "SELECT * FROM table1",
			want:  "SELECT * FROM table1",
		},
		{
			name:  "multiple values in an insert",
			query: "INSERT INTO t (a, b) VALUES (1, 2)",
			want:  "INSERT INTO t (a, b) VALUES (?, ?)",
		},
		{
			name:  "identifier with a trailing digit is untouched",
			query: "SELECT col1 FROM t2",
			want:  "SELECT col1 FROM t2",
		},
		{
			name:  "escaped quote inside a string literal",
			query: `SELECT * FROM t WHERE name = 'O\'Brien'`,
			want:  "SELECT * FROM t WHERE name = ?",
		},
		{
			name:  "apostrophe inside a double-quoted identifier does not desync later literals",
			query: `SELECT * FROM t AS "employee's_table" WHERE email = 'jane.doe@example.com'`,
			want:  `SELECT * FROM t AS "employee's_table" WHERE email = ?`,
		},
		{
			name:  "apostrophe inside a line comment does not desync later literals",
			query: "SELECT * FROM t -- don't leak this\nWHERE ssn = '123-45-6789'",
			want:  "SELECT * FROM t -- don't leak this\nWHERE ssn = ?",
		},
		{
			name:  "apostrophe inside a block comment does not desync later literals",
			query: "SELECT * FROM t /* it's a block comment */ WHERE name = 'bob'",
			want:  "SELECT * FROM t /* it's a block comment */ WHERE name = ?",
		},
		{
			name:  "hexadecimal literal is masked",
			query: "SELECT * FROM t WHERE flags = 0x1F",
			want:  "SELECT * FROM t WHERE flags = ?",
		},
		{
			name:  "scientific notation literal is masked",
			query: "SELECT * FROM t WHERE ratio = 1.5e-10",
			want:  "SELECT * FROM t WHERE ratio = ?",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskSQL(tc.query); got != tc.want {
				t.Errorf("MaskSQL(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestApplyHeaderRule_Disabled(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	_, span := tp.Tracer("test").Start(context.Background(), "op")

	ApplyHeaderRule(span, "http.request.header.", map[string][]string{"X-Test": {"v"}}, HeaderRule{Enabled: false}, false)
	span.End()

	if len(exp.GetSpans()[0].Attributes) != 0 {
		t.Error("expected no attributes when the rule is disabled")
	}
}

func TestApplyHeaderRule_OnErrorOnly(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	rule := HeaderRule{Enabled: true, OnErrorOnly: true}

	_, span := tp.Tracer("test").Start(context.Background(), "success")
	ApplyHeaderRule(span, "http.request.header.", map[string][]string{"X-Test": {"v"}}, rule, false)
	span.End()

	_, span2 := tp.Tracer("test").Start(context.Background(), "failure")
	ApplyHeaderRule(span2, "http.request.header.", map[string][]string{"X-Test": {"v"}}, rule, true)
	span2.End()

	spans := exp.GetSpans()
	if len(spans[0].Attributes) != 0 {
		t.Error("expected no attributes on a successful call with OnErrorOnly set")
	}
	if len(spans[1].Attributes) != 1 {
		t.Error("expected an attribute on a failed call with OnErrorOnly set")
	}
}

func TestApplyHeaderRule_ExcludeAndMask(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	rule := HeaderRule{
		Enabled: true,
		Exclude: []string{"Authorization"},
		Mask:    []string{"X-Api-Key"},
	}

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	ApplyHeaderRule(span, "http.request.header.", map[string][]string{
		"Authorization": {"Bearer secret"},
		"X-Api-Key":     {"key-123"},
		"X-Plain":       {"visible"},
	}, rule, false)
	span.End()

	attrs := exp.GetSpans()[0].Attributes
	found := map[string]string{}
	for _, kv := range attrs {
		found[string(kv.Key)] = kv.Value.AsStringSlice()[0]
	}
	if _, ok := found["http.request.header.authorization"]; ok {
		t.Error("Authorization must never be captured (Exclude)")
	}
	if got := found["http.request.header.x-api-key"]; got != maskedValue {
		t.Errorf("X-Api-Key = %q, want masked value %q", got, maskedValue)
	}
	if got := found["http.request.header.x-plain"]; got != "visible" {
		t.Errorf("X-Plain = %q, want unmasked %q", got, "visible")
	}
}
