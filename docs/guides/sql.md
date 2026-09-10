# SQL

`integrations/sql` instruments any `database/sql` driver generically via the
`database/sql/driver` interfaces - the same wrapper works for Postgres,
MySQL, SQLite, SQL Server, etc., no per-driver code. It covers both the
modern context-aware driver interfaces and the legacy fallback path
`database/sql` itself uses when a driver doesn't implement them.

```go
import (
	_ "github.com/jackc/pgx/v5/stdlib"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	argossql "github.com/jhonsferg/argos/integrations/sql"
)

db, err := argossql.Open("pgx", dsn, argossql.WithSystem(semconv.DBSystemPostgreSQL))
```

`db` is a real `*sql.DB` - use it exactly as you would otherwise.

## Options

- `WithSystem(attribute.KeyValue)` - sets `db.system` on every span/metric
  (e.g. `semconv.DBSystemPostgreSQL`). Optional; omitting it just means
  spans won't carry that attribute.
- `WithQueryText(bool)` - records the query text as `db.query.text`. **Off
  by default** - query text can embed literal parameter values (PII,
  secrets).
- `WithMaskedQueryText(bool)` - records the query text with string and
  numeric literals replaced by `?` (e.g. `WHERE id=? AND name=?`), via
  `capture.MaskSQL`. A middle ground between off and raw: the query
  _shape_ is visible for debugging without the literal values. If both
  this and `WithQueryText` are enabled, the masked version wins.

  `capture.MaskSQL` is a best-effort regex-based redactor, not a real SQL
  parser, and its guarantees have been verified by sustained fuzz testing
  (see `core/capture/fuzz_test.go` and `core/capture/testdata/fuzz/`):

  - Single-quoted string literals (including escaped quotes, embedded
    JSON, unicode, and multi-line values) and numeric literals - decimal,
    hexadecimal (`0x1F`), and scientific notation (`1.5e-10`) - are
    replaced with `?`.
  - An apostrophe inside a `--`/`/* */` comment or a double-quoted
    identifier (e.g. `"employee's_table"`) does not desynchronize masking
    for the rest of the query - this was a real bug fuzzing found and
    fixed, not an assumption.
  - **Known limitation**: `MaskSQL` assumes syntactically valid SQL/CQL. A
    bare, unbalanced quote outside of a comment or a double-quoted
    identifier (malformed input) can still desynchronize masking for
    everything after it in the query.
  - **Known limitation**: content inside a comment is treated as static
    text, not data, and is left unmasked on purpose (so annotations like
    sqlcommenter tags survive) - do not embed sensitive dynamic values in
    a SQL comment if you rely on `WithMaskedQueryText`.
  - **Known limitation**: an unquoted value that doesn't parse as a single
    numeric literal - most notably a raw, hyphen-separated UUID
    (`550e8400-e29b-...`) - may only be partially masked. Values coming
    through bound parameter placeholders are unaffected since they never
    appear in the query text at all; this only matters if you build query
    text via string concatenation.

- `WithLogger(logging.Logger)` - logs query failures in addition to the
  span/metric recording that always happens. Optional even without this -
  see [Logging](logging.md#global-ambient-logging) for the global default
  every integration falls back to.
- `WithYAMLConfig(cfg core.Config)` - applies the `integrations.sql` section
  of a config loaded via `argos.WithYAMLConfig`/`argos.FromYAML`
  (`query_text`, `query_text_masked`). See [Configuration](configuration.md)
  for the full YAML shape and how it layers with the options above.

## See it running

[`orders-api`](https://github.com/jhonsferg/argos/tree/main/samples/orders-api)
uses `argossql.Open` directly against Postgres with a Redis cache-aside layer
in front of reads.
