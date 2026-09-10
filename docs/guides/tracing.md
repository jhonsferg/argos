# Tracing custom operations

Every integration in this library instruments a specific third-party SDK
call. `argos.Trace`/`argos.TraceFunc` do the same thing for your own
business logic - a repository method, a use case, anything worth its own
span - without a struct field or a per-return-shape constructor.

```go
order, err := argos.Trace(ctx, "orders.FindByID", func(ctx context.Context) (*Order, error) {
	return repo.findByID(ctx, id)
})
```

`Trace[T]` is generic over the return type, so it works the same whether
the wrapped function returns a single value, a slice, or nothing useful
besides an error. For the last case, `TraceFunc` skips the unused return
value:

```go
err := argos.TraceFunc(ctx, "orders.Delete", func(ctx context.Context) error {
	return repo.delete(ctx, id)
})
```

## What it does

- Starts an `INTERNAL`-kind span named by the string you pass, as a child
  of whatever span is already active in `ctx`.
- On error: records it on the span (`span.RecordError` +
  `codes.Error`) and logs it through `core/log`'s global Logger
  (`argoslog.Error(ctx, "operation failed", err, argoslog.F("operation", name))`) -
  no logger instance needs to reach the call site, see
  [Logging](logging.md#global-ambient-logging).
- Records the call's duration on an `operation.duration` histogram,
  tagged with an `operation` attribute - the same "one histogram per
  module" convention every integration in this repo follows.
- Returns `fn`'s result and error completely unchanged otherwise.

## Adding attributes inside fn

`Trace` doesn't expose the span directly in its signature, since most calls
don't need it - reach it from `ctx` when one does:

```go
order, err := argos.Trace(ctx, "orders.FindByID", func(ctx context.Context) (*Order, error) {
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("order.id", id))
	return repo.findByID(ctx, id)
})
```
