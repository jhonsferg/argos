# Logging

`core.Logger` (an alias for `core/logging.Logger`) is the interface every
integration in this library depends on for error/failure logging - never a
concrete logging library. The default implementation is zerolog-backed
(`logging.NewZerolog`), chosen for its zero-allocation JSON logging, and is
what `argos.Init` wires up automatically:

```go
provider, _ := argos.Init(ctx, argos.WithServiceName("my-service"))
logger := provider.Logger()

logger.Info(ctx, "handling request", argos.F("user_id", userID))
logger.Error(ctx, "request failed", err)
```

Every log call automatically reads `trace_id`/`span_id` off the passed
`context.Context` and attaches them - callers never do this themselves.

## Bringing your own logger

Implement `core.Logger` (`Debug`/`Info`/`Warn`/`Error`/`With`, all taking a
`context.Context` first) and pass it via `argos.WithLogger(yourLogger)`. The
core and every integration only depend on the interface, never on zerolog
directly, so swapping in `slog` or `zap` behind an adapter doesn't touch any
other code.

## Global ambient logging

`provider.Logger()` is still there for callers that want an explicit
instance, but most code doesn't need to carry one through constructors at
all. `core/log` gives every layer of an application - a repository, a
background worker, a package with no idea an `argos.Provider` even exists -
ambient access to the same Logger `argos.Init` configured, the same way
`otel.Tracer(name)`/`otel.Meter(name)` are already ambient:

```go
package repository

import (
	"context"

	argoslog "github.com/jhonsferg/argos/log"
)

func (r *OrderRepository) FindByID(ctx context.Context, id string) (*Order, error) {
	order, err := r.query(ctx, id)
	if err != nil {
		argoslog.Error(ctx, "order lookup failed", err, argoslog.F("order_id", id))
		return nil, err
	}
	return order, nil
}
```

No `Logger` instance is threaded through `NewOrderRepository(...)` - `argos.Init`
calls `argoslog.SetDefault` internally, so every `argoslog.Info`/`Warn`/`Error`/`Debug`
call anywhere in the process reaches the same configured Logger, correlated
with `ctx`'s trace/span exactly like `provider.Logger()` would be. Before
`argos.Init` runs (a package `init()`, or a test that never bootstraps
argos), `core/log` falls back to a no-op Logger - calls are silently
dropped rather than panicking.

Every built-in integration (`sql`, `redis`, `gorm`, ...) already defaults
its own `WithLogger` option to `core/log`'s current default when none is
passed explicitly, so error-level logging for a failed query/publish/call
"just works" without any extra wiring.

Import it under an alias (`argoslog` above) if the file already imports the
stdlib `log` package, since the import path's package name is also `log`.
