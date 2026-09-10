// Package capture holds shared, reusable building blocks for the opt-in
// "capture sensitive data" options every integration in this repo exposes
// (query text, request/response bodies, headers/metadata) - one place for
// the exclude/mask/size-limit rules and the OTel attribute mechanics,
// instead of every integration re-inventing them.
package capture

import (
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Rule gates capturing a body (request or response payload).
type Rule struct {
	// Enabled turns capture on at all. Off by default in every integration
	// that uses this - bodies can carry PII/secrets.
	Enabled bool
	// MaxBytes bounds how much of the body is captured. Callers are
	// responsible for actually truncating to this bound; Rule itself is
	// just the declared policy.
	MaxBytes int
	// OnErrorOnly, when true, restricts capture to calls that ended in an
	// error (or, for HTTP, a >=500 response) - never on success.
	OnErrorOnly bool
}

// HeaderRule gates capturing a set of headers (HTTP) or metadata (gRPC).
type HeaderRule struct {
	Enabled     bool
	OnErrorOnly bool
	// Exclude lists header/metadata names (case-insensitive) that are
	// never captured, regardless of Enabled - use this for anything that
	// must never leave the process (e.g. "Authorization", "Cookie").
	Exclude []string
	// Mask lists header/metadata names (case-insensitive) that are
	// captured, but with their value replaced by a fixed mask - use this
	// for values worth knowing were present without exposing their
	// content (e.g. "X-Api-Key").
	Mask []string
}

// HTTPRules bundles the four capture points a request/response cycle has.
// Reused as-is by integrations/httpclient and integrations/httpserver/core
// (HTTP headers/bodies) and integrations/grpc (gRPC metadata standing in
// for headers, the marshaled message standing in for the body).
type HTTPRules struct {
	RequestBody     Rule
	ResponseBody    Rule
	RequestHeaders  HeaderRule
	ResponseHeaders HeaderRule
}

const maskedValue = "***"

// ApplyHeaderRule sets one span attribute per entry in values (attribute
// name: prefix + the lowercased header/metadata name), skipping entries
// named in rule.Exclude entirely and replacing the value of entries named
// in rule.Mask with a fixed mask. It does nothing unless rule.Enabled and
// (!rule.OnErrorOnly || isError).
func ApplyHeaderRule(span trace.Span, prefix string, values map[string][]string, rule HeaderRule, isError bool) {
	if !rule.Enabled || (rule.OnErrorOnly && !isError) {
		return
	}
	exclude := toLowerSet(rule.Exclude)
	mask := toLowerSet(rule.Mask)

	for name, vals := range values {
		lower := strings.ToLower(name)
		if exclude[lower] {
			continue
		}
		attrName := prefix + lower
		if mask[lower] {
			span.SetAttributes(attribute.StringSlice(attrName, []string{maskedValue}))
			continue
		}
		span.SetAttributes(attribute.StringSlice(attrName, vals))
	}
}

func toLowerSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[strings.ToLower(n)] = true
	}
	return set
}

var (
	// sqlToken matches, in priority order, the four constructs that can
	// legally contain a quote character without it terminating a string
	// literal: a line comment, a block comment, a double-quoted
	// identifier, and finally the single-quoted string literal itself.
	// Matching the first three as atomic units - and leaving them
	// untouched - keeps a stray apostrophe inside a comment or a quoted
	// identifier (e.g. "-- don't do this" or `"employee's_table"`) from
	// desynchronizing the quote pairing for the rest of the query, which
	// would otherwise let a real literal further along leak unmasked.
	sqlToken         = regexp.MustCompile(`--[^\n]*|/\*(?:[^*]|\*[^/])*\*/|"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)
	sqlNumberLiteral = regexp.MustCompile(`\b(?:0[xX][0-9a-fA-F]+|\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\b`)
)

// MaskSQL is a best-effort SQL/CQL literal redactor: it replaces
// single-quoted string literals and bare numeric literals with "?",
// leaving the query's shape (keywords, identifiers, structure) intact -
// e.g. "SELECT * FROM t WHERE id=5 AND name='bob'" becomes
// "SELECT * FROM t WHERE id=? AND name=?". This is a regex, not a real SQL
// parser: it assumes syntactically valid SQL/CQL. Comments and
// double-quoted identifiers are recognized and left untouched even when
// they contain an apostrophe, but arbitrary malformed input with a bare,
// unbalanced quote outside of those constructs can still desynchronize
// masking for the remainder of the query - the same "documented best
// effort" spirit as every other opt-in sensitive-attribute feature in this
// repo.
func MaskSQL(query string) string {
	masked := sqlToken.ReplaceAllStringFunc(query, func(tok string) string {
		if strings.HasPrefix(tok, "'") {
			return "?"
		}
		return tok
	})
	masked = sqlNumberLiteral.ReplaceAllString(masked, "?")
	return masked
}
