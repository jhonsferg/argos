# Testing

`core/argostest` provides the two fixtures every module's own tests in this
repo otherwise hand-roll: an in-memory span exporter installed as the global
`TracerProvider`, and a `Logger` that records calls instead of writing
anywhere, installed as [`core/log`](logging.md#global-ambient-logging)'s
global default. Use it to test your own code that's instrumented with
Argos - a handler, a repository wrapped in `argos.Trace`, anything that
starts spans or logs through the global logger.

```go
import "github.com/jhonsferg/argos/argostest"

func TestOrderRepository_FindByID(t *testing.T) {
	exp := argostest.NewTracer(t)
	log := argostest.NewLogger(t)

	_, err := repo.FindByID(context.Background(), "missing-id")
	if err == nil {
		t.Fatal("expected an error")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Errorf("expected 1 error span, got %+v", spans)
	}

	entries := log.Entries()
	if len(entries) != 1 || entries[0].Level != "error" {
		t.Errorf("expected 1 error log entry, got %+v", entries)
	}
}
```

## `NewTracer`

Installs a real SDK `TracerProvider` backed by an in-memory exporter as the
global `TracerProvider` for the duration of the test, restoring whatever
was previously installed via `t.Cleanup`. Read spans back with the returned
`*tracetest.InMemoryExporter`'s `GetSpans()`.

## `NewLogger`

Installs a `*argostest.RecordingLogger` as `core/log`'s global default for
the duration of the test (also restored via `t.Cleanup`). Every
`Debug`/`Info`/`Warn`/`Error` call - including ones made through a scoped
Logger built with `.With(...)` - is recorded and available via
`Entries()`, in order, each carrying its level, message, error (if any),
and structured fields.

## Why not just check span attributes for everything

A `RecordingLogger` matters specifically because `core/log`'s ambient
logging (and `argos.Trace`'s built-in error logging) happens through the
_global_ default, not an instance your test already holds - without
installing one, there's nothing to assert against. `NewTracer` exists for
the same reason on the tracing side, even though `otel/sdk/trace/tracetest`
is public API - it saves re-writing the same six lines of
provider-swap-and-restore boilerplate every test file in this repo used to
carry.
