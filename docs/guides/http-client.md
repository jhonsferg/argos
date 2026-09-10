# HTTP Client

`integrations/httpclient` wraps an `http.RoundTripper` - a drop-in
`Transport`, the same shape as the ecosystem's own `otelhttp.NewTransport`.
It records a client span and duration metric per request and injects the
active trace context into outgoing headers.

```go
import argoshttpclient "github.com/jhonsferg/argos/integrations/httpclient"

client := argoshttpclient.NewClient(http.DefaultClient)
resp, err := client.Get("https://api.example.com/widgets")
```

Or wrap just the transport if you already have a customized `*http.Client`:

```go
client := &http.Client{Timeout: 5 * time.Second}
client.Transport = argoshttpclient.Wrap(client.Transport)
```

`next` defaults to `http.DefaultTransport` if nil.

## Capturing bodies on error

`WithCaptureBodyOnError(maxBytes)` attaches up to `maxBytes` of the request
and response bodies to the span (`argos.http.request.body`/
`argos.http.response.body`) only when the call errors or the response
status is `>= 500` - never on success. Off by default, same PII rationale
as the [HTTP Servers](http-servers.md#capturing-bodies-on-error) capture
option:

```go
client := argoshttpclient.NewClient(http.DefaultClient, argoshttpclient.WithCaptureBodyOnError(4096))
```

Capturing never truncates what's actually sent/received - both bodies are
read back in full by the real transport/caller regardless of `maxBytes`.

## Capture rules

`WithCaptureBodyOnError` is sugar for the more general `WithCapture(rules
capture.HTTPRules)`, which independently controls request/response bodies
**and** headers:

```go
import "github.com/jhonsferg/argos/capture"

client := argoshttpclient.NewClient(http.DefaultClient, argoshttpclient.WithCapture(capture.HTTPRules{
	RequestBody:  capture.Rule{Enabled: true, MaxBytes: 4096, OnErrorOnly: true},
	ResponseBody: capture.Rule{Enabled: true, MaxBytes: 4096, OnErrorOnly: true},
	RequestHeaders: capture.HeaderRule{
		Enabled: true,
		Exclude: []string{"Authorization"},
		Mask:    []string{"X-Api-Key"},
	},
	ResponseHeaders: capture.HeaderRule{Enabled: true},
}))
```

- `Rule.OnErrorOnly` gates a body the same way `WithCaptureBodyOnError`
  does (error or `>= 500` only) - set it `false` to capture on every call.
- `HeaderRule.Exclude`/`Mask` are case-insensitive name lists: excluded
  headers are never captured, masked ones are captured with the value
  replaced by `***`. Both default to nil (capture everything not excluded).
- Captured headers land as `http.request.header.<lowercased-name>` /
  `http.response.header.<lowercased-name>` (array-valued, following the
  OTel semconv pattern for HTTP headers).

`WithYAMLConfig(cfg core.Config)` applies the `integrations.httpclient`
section of a config loaded via `argos.WithYAMLConfig`/`argos.FromYAML`:

```yaml
integrations:
  httpclient:
    capture:
      request_body: { enabled: true, max_bytes: 4096, on_error_only: true }
      response_body: { enabled: true, max_bytes: 4096, on_error_only: true }
      request_headers:
        enabled: true
        exclude: ["Authorization"]
        mask: ["X-Api-Key"]
      response_headers:
        enabled: true
```

It composes with `Option`s applied elsewhere in the chain - see
[Configuration](configuration.md).
