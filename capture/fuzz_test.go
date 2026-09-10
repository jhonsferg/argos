package capture

import (
	"context"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// FuzzMaskSQL checks that MaskSQL never panics and never grows its input -
// every match it replaces is collapsed to the single character "?", so the
// masked output can never be longer than the query it came from - and that
// masking an already-masked query is a no-op (running MaskSQL twice must
// equal running it once).
func FuzzMaskSQL(f *testing.F) {
	seeds := []string{
		// Real shapes from samples/*/store.go.
		"INSERT INTO orders (customer_name, item, quantity) VALUES ($1, $2, $3)",
		"SELECT id, customer_name, item, quantity, created_at FROM orders WHERE id = $1",
		"INSERT INTO items (id, name, stock) VALUES ($1, $2, $3)",
		"SELECT id, name, stock FROM items WHERE id = $1",
		"UPDATE items SET stock = stock - $1 WHERE id = $2 AND stock >= $1",
		"INSERT INTO reports (filename, size_bytes, pulled_at) VALUES (?, ?, ?)",
		"SELECT filename, size_bytes, pulled_at FROM reports WHERE filename = ?",
		"INSERT INTO stock_changes (item_id, changed_at, remaining) VALUES (?, ?, ?)",
		// Adversarial cases.
		`SELECT * FROM t WHERE name = 'O\'Brien'`,
		`SELECT * FROM t WHERE name = 'it''s escaped the sql-standard way'`,
		"SELECT * FROM t -- WHERE secret = 'shhh'",
		"SELECT * FROM t /* block comment with 'a literal' inside */",
		"SELECT * FROM t WHERE flags = 0x1F",
		"SELECT * FROM t WHERE ratio = 1.5e-10",
		"SELECT * FROM t WHERE ratio = 1E10",
		"SELECT * FROM t WHERE id IN ({1,2,3})",
		"SELECT * FROM t WHERE tags CONTAINS {'a': 1, 'b': 2}",
		`SELECT * FROM t WHERE data = '{"a":1,"b":"c"}'`,
		"SELECT * FROM t WHERE name = 'café éè'",
		"",
		"'",
		"''",
		"SELECT *\nFROM t\nWHERE name = 'multi\nline\nvalue'",
		"SELECT * FROM t WHERE id = 550e8400-e29b-41d4-a716-446655440000",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, query string) {
		got := MaskSQL(query)

		if len(got) > len(query) {
			t.Fatalf("MaskSQL grew the input: query=%q (%d bytes) got=%q (%d bytes)",
				query, len(query), got, len(got))
		}

		if again := MaskSQL(got); again != got {
			t.Fatalf("MaskSQL is not idempotent: MaskSQL(%q) = %q, MaskSQL(that) = %q", query, got, again)
		}
	})
}

// FuzzMaskSQL_LiteralLeakage builds a query with a deterministically placed
// single-quoted string literal around a fuzzed value, and checks that the
// literal's content never survives masking - the core promise
// WithMaskedQueryText makes to callers. The property only applies when
// MaskSQL's own tokenizer recognizes the constructed literal as a literal
// in the first place: surrounding text can legitimately absorb it instead
// (a preceding unterminated comment swallows the rest of the line by
// design - see MaskSQL's doc comment - and a bare unbalanced quote outside
// a comment or identifier is malformed SQL, outside MaskSQL's contract).
func FuzzMaskSQL_LiteralLeakage(f *testing.F) {
	type seed struct{ prefix, secret, suffix string }
	seeds := []seed{
		{"SELECT * FROM t WHERE name = ", "bob", ""},
		{"SELECT * FROM t WHERE ssn = ", "123-45-6789", " AND active = true"},
		{"UPDATE t SET note = ", "O'Brien", " WHERE id = 1"},
		{"SELECT * FROM t WHERE payload = ", `{"card":"4111111111111111"}`, ""},
		{"SELECT * FROM t WHERE name = ", "café", ""},
		{"SELECT * FROM t WHERE token = ", `back\slash`, ""},
		{"", "", ""},
	}
	for _, s := range seeds {
		f.Add(s.prefix, s.secret, s.suffix)
	}

	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`)

	f.Fuzz(func(t *testing.T, prefix, secret, suffix string) {
		escaped := replacer.Replace(secret)
		// Degenerate secrets that can't leak meaningfully: empty, or
		// containing "?" itself - the mask sentinel every masked token in
		// the whole query produces, so a secret containing it can
		// coincidentally reassemble from two unrelated masked/unmasked
		// fragments sitting next to each other with no actual value
		// exposed (e.g. an unmasked digit run glued to a preceding
		// identifier, immediately followed by an unrelated masked "?").
		if strings.TrimSpace(escaped) == "" || strings.Contains(escaped, "?") {
			return
		}
		// Skip seeds where the secret already appears outside the
		// literal - any "leak" found would be a false positive coming
		// from prefix/suffix, not from MaskSQL failing to mask it.
		if strings.Contains(prefix, escaped) || strings.Contains(suffix, escaped) {
			return
		}

		query := prefix + "'" + escaped + "'" + suffix
		literalStart, literalEnd := len(prefix), len(prefix)+1+len(escaped)+1
		recognized := false
		for _, m := range sqlToken.FindAllStringIndex(query, -1) {
			if m[0] == literalStart && m[1] == literalEnd {
				recognized = true
				break
			}
		}
		if !recognized {
			return
		}

		got := MaskSQL(query)
		if strings.Contains(got, escaped) {
			t.Fatalf("literal value survived masking: query=%q got=%q secret=%q", query, got, secret)
		}
	})
}

// FuzzApplyHeaderRule_Exclude checks that a header named in Exclude never
// appears as a span attribute, regardless of the capitalization used for
// either the rule entry or the actual header name.
func FuzzApplyHeaderRule_Exclude(f *testing.F) {
	type seed struct{ excludeName, headerName, value string }
	seeds := []seed{
		{"Authorization", "Authorization", "Bearer secret"},
		{"authorization", "AUTHORIZATION", "Bearer secret"},
		{"X-Api-Key", "x-api-key", "key-123"},
		{"", "", ""},
		{"X-Ünicode", "x-Ünicode", "v"},
		{strings.Repeat("a", 5000), strings.Repeat("a", 5000), "v"},
	}
	for _, s := range seeds {
		f.Add(s.excludeName, s.headerName, s.value)
	}

	f.Fuzz(func(t *testing.T, excludeName, headerName, value string) {
		if excludeName == "" || headerName == "" {
			return
		}

		exp := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
		rule := HeaderRule{Enabled: true, Exclude: []string{excludeName}}

		_, span := tp.Tracer("fuzz").Start(context.Background(), "op")
		ApplyHeaderRule(span, "http.request.header.", map[string][]string{headerName: {value}}, rule, false)
		span.End()

		attrName := "http.request.header." + strings.ToLower(headerName)
		var leaked bool
		for _, kv := range exp.GetSpans()[0].Attributes {
			if string(kv.Key) == attrName {
				leaked = true
			}
		}

		if strings.EqualFold(excludeName, headerName) && leaked {
			t.Fatalf("excluded header leaked: exclude=%q actual=%q", excludeName, headerName)
		}
	})
}

// FuzzApplyHeaderRule_Mask checks that a header named in Mask never exposes
// its real value, regardless of the capitalization used for either the
// rule entry or the actual header name.
func FuzzApplyHeaderRule_Mask(f *testing.F) {
	type seed struct{ maskName, headerName, value string }
	seeds := []seed{
		{"X-Api-Key", "X-Api-Key", "key-123"},
		{"x-api-key", "X-API-KEY", "key-123"},
		{"", "", ""},
		{"X-Ünicode", "x-Ünicode", "secret-value"},
		{strings.Repeat("a", 5000), strings.Repeat("a", 5000), "secret-value"},
	}
	for _, s := range seeds {
		f.Add(s.maskName, s.headerName, s.value)
	}

	f.Fuzz(func(t *testing.T, maskName, headerName, value string) {
		if maskName == "" || headerName == "" || value == "" {
			return
		}
		if !strings.EqualFold(maskName, headerName) {
			return
		}

		exp := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
		rule := HeaderRule{Enabled: true, Mask: []string{maskName}}

		_, span := tp.Tracer("fuzz").Start(context.Background(), "op")
		ApplyHeaderRule(span, "http.request.header.", map[string][]string{headerName: {value}}, rule, false)
		span.End()

		attrName := "http.request.header." + strings.ToLower(headerName)
		for _, kv := range exp.GetSpans()[0].Attributes {
			if string(kv.Key) != attrName {
				continue
			}
			for _, v := range kv.Value.AsStringSlice() {
				if v == value {
					t.Fatalf("masked header exposed real value: name=%q value=%q", headerName, value)
				}
			}
		}
	})
}
