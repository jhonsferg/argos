# Writing a new Argos integration

This module is a worked template, not a real integration: `example.go`
wraps a fake `WidgetClient` the same way a real module in this repo wraps a
real SDK. Copy its shape, not its code, once you know which of the three
cases below your target library falls into.

Every integration in this repo follows the same shared conventions
regardless of shape:

- A `Wrap`/free-function constructor takes `...Option`; the only option
  every module has is `WithLogger(logging.Logger)`.
- One span per operation, `trace.WithSpanKind(trace.SpanKindClient)` (or
  `SpanKindServer`/`SpanKindProducer`/`SpanKindConsumer` where relevant),
  named after the operation.
- One `Float64Histogram` metric (`<system>.client.operation.duration`,
  unit `s`), created once, recorded on every call regardless of outcome.
- Errors: `span.RecordError` + `span.SetStatus(codes.Error, ...)`, plus a
  `logger.Error` call if a logger was configured.
- Anything that could leak sensitive data (a SQL query, a file path, an
  email recipient) is opt-in via an `Option`, off by default. For a
  request/response-shaped protocol (HTTP, gRPC, any RPC with headers or
  metadata and a body), reuse `core/capture` rather than hand-rolling
  truncation/masking again: `capture.Rule`/`capture.HeaderRule` gate a
  body/header set by enabled/size/on-error-only, `capture.HTTPRules`
  bundles the four capture points a request/response cycle has (request
  and response, body and headers), `capture.ApplyHeaderRule` does the
  actual exclude/mask/attribute-setting work, and `capture.MaskSQL` is a
  regex-based literal redactor useful for any SQL/CQL-shaped query text.
  See `integrations/httpclient`, `integrations/httpserver/core`, and
  `integrations/grpc`'s `UnaryClientInterceptor` for three real usages, and
  `core/capture/yaml.go` (`YAMLRule`/`YAMLHeaderRule`/`YAMLHTTPRules`) for
  the matching YAML mirror types to use in `WithYAMLConfig` instead of
  writing a bespoke one.
- Tests: a fast unit test with no real network/Docker dependency, a
  comparative benchmark (`Benchmark..._Baseline` vs
  `Benchmark..._Instrumented`, `-benchmem`, documenting the allocation cost
  the wrapper adds), and, where a real server is feasible to run via
  testcontainers-go, a `//go:build integration` test against it.

## Which shape is your library?

**1. Does it expose an interface (or can you define one that covers the
methods you need), and do those methods already take `context.Context`?**

That's this module's case - the simplest one. Wrap the interface directly,
as `example.go` does: `Wrap(next Interface, opts ...Option) Interface`,
delegate to `next` inside each method, span/metric/log around the call.
Callers depend on the interface, so nothing about their code changes
besides the constructor call. Most gRPC-generated clients and many REST
SDKs have this shape.

**2. Is it a concrete struct (no interface) with no `context.Context`
parameter, and no cross-process trace propagation to worry about?**

See `integrations/sftp` (`argossftp`, wrapping `*sftp.Client`) or
`integrations/rabbitmq`/`integrations/azuresb` (wrapping RabbitMQ's
`Channel`/`Delivery` and Azure Service Bus's `Sender`/`ReceivedMessage`).
Two ways to add spans without a context parameter to hang them on:

- If the type is a concrete struct, embed it: `type Client struct {
*sftp.Client; cfg *config }`. Every method you don't override keeps
  working via Go's method promotion - you only write overrides for the
  handful of operations worth a span (see `argossftp`'s
  `client.go`), each added as a new `*Context`-suffixed method
  (`OpenContext(ctx, path)`) rather than silently changing an existing
  method's signature.
- If there's no single type to embed at all (e.g. a package-level
  function like `net/smtp.SendMail`, or a sync producer with no
  wrappable client type), wrap it as a free function instead:
  `SendMail(ctx context.Context, addr string, ...) error` - see
  `integrations/smtp` (`argossmtp`).

**3. Does it involve message-passing where the trace context itself has
to travel with the message (producer/consumer, pub/sub)?**

See `integrations/messaging/core` - the shared `Instrumentor` used by
`argoskafka`, `argosrabbitmq`, `argosazuresb`, and `argosgcppubsub`. The
key piece is a small `propagation.TextMapCarrier` adapter over whatever
the library uses for message headers/properties (a `map[string]string`,
an `amqp.Table`, `azservicebus.Message.ApplicationProperties`, ...) so
`otel.GetTextMapPropagator().Inject`/`Extract` can read and write it. The
producer side injects before sending; the consumer side extracts before
starting its span, so the consumer span is correctly linked to the
producer span that sent the message.

## Optional: YAML-driven configuration

Any Option can also be made settable from an argos YAML config file (loaded
via `argos.WithYAMLConfig`/`argos.FromYAML`), without core needing to know
anything about the module. `core.Config.Integrations` holds one raw YAML
subtree per module, keyed by the module's own name; `core.DecodeIntegrationConfig`
decodes it on demand. `integrations/sql/yaml.go` is the worked example - the
shape to copy:

```go
type yamlConfig struct {
	// Pointer fields distinguish "absent from YAML" from "explicitly false",
	// so an absent key never overrides an Option applied elsewhere in the
	// chain.
	SomeFlag *bool `yaml:"some_flag"`
}

func WithYAMLConfig(cfg core.Config) Option {
	return func(c *config) {
		var y yamlConfig
		ok, err := core.DecodeIntegrationConfig(cfg, "yourmodulekey", &y)
		if err != nil || !ok {
			return
		}
		if y.SomeFlag != nil {
			c.someFlag = *y.SomeFlag
		}
	}
}
```

This is opt-in per module - only add it for fields worth toggling outside
of Go code (the sensitive-data-carrying attributes described above are
prime candidates). It composes with the rest of the Options chain rather
than replacing it: `Wrap(next, WithYAMLConfig(cfg), WithSomething(true))`
still lets `WithSomething` override whatever the YAML section set.

## If none of these fit

Stop and ask before inventing a fourth shape - it usually means the
library has a quirk (streaming, bidirectional RPC, a driver-level
`database/sql` integration like `integrations/sql`) that deserves its
own design conversation rather than a forced fit into one of the above.
