# HTTP Servers

Each router has a thin adapter over a shared core
(`integrations/httpserver/core`) that resolves `http.route`, status, and
error classification once, so that logic isn't duplicated six times. Every
adapter's `Middleware()` returns the router's own native middleware type -
there's no bespoke API to learn per framework.

net/http-family routers (net/http itself, chi, gorilla/mux) resolve the
matched route _after_ calling the next handler, since the stdlib router
doesn't expose it beforehand. Routers with their own request context (gin,
echo, fiber) already know the route before the handler runs.

=== "net/http"

    ```go
    import argosnethttp "github.com/jhonsferg/argos/integrations/httpserver/nethttp"

    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", handler)
    handler := argosnethttp.Middleware()(mux)
    http.ListenAndServe(":8080", handler)
    ```

=== "chi"

    ```go
    import argoschi "github.com/jhonsferg/argos/integrations/httpserver/chi"

    r := chi.NewRouter()
    r.Use(argoschi.Middleware())
    r.Get("/health", handler)
    ```

=== "gin"

    ```go
    import argosgin "github.com/jhonsferg/argos/integrations/httpserver/gin"

    r := gin.New()
    r.Use(argosgin.Middleware())
    r.GET("/health", handler)
    ```

=== "echo"

    ```go
    import argosecho "github.com/jhonsferg/argos/integrations/httpserver/echo"

    e := echo.New()
    e.Use(argosecho.Middleware())
    e.GET("/health", handler)
    ```

=== "fiber"

    ```go
    import argosfiber "github.com/jhonsferg/argos/integrations/httpserver/fiber"

    app := fiber.New()
    app.Use(argosfiber.Middleware())
    app.Get("/health", handler)
    ```

    Fiber runs on fasthttp, not `net/http` - inside a handler, read the
    span-carrying context via `c.UserContext()` (set by the middleware),
    not the raw `c.Context()`.

=== "gorilla/mux"

    ```go
    import argosgorillamux "github.com/jhonsferg/argos/integrations/httpserver/gorillamux"

    r := mux.NewRouter()
    r.Use(argosgorillamux.Middleware())
    r.HandleFunc("/health", handler)
    ```

## Capturing bodies on error

`httpservercore.WithCaptureBodyOnError(maxBytes)` (forwarded by every
adapter's `Middleware(opts ...)`) attaches up to `maxBytes` of the request
and response bodies to the span as `argos.http.request.body`/
`argos.http.response.body` - but **only** when the response status is
`>= 500`. A successful request never pays for or carries this attribute.
Off by default: bodies can carry PII or secrets, so this is an explicit,
bounded opt-in, not a default-on attribute.

```go
argosnethttp.Middleware(httpservercore.WithCaptureBodyOnError(4096))
```

## Capture rules

`WithCaptureBodyOnError` is sugar for the more general
`httpservercore.WithCapture(rules capture.HTTPRules)`, which independently
controls request/response bodies **and** headers - same shape as
[HTTP Client](http-client.md#capture-rules):

```go
import "github.com/jhonsferg/argos/capture"

argosnethttp.Middleware(httpservercore.WithCapture(capture.HTTPRules{
	RequestBody:  capture.Rule{Enabled: true, MaxBytes: 4096, OnErrorOnly: true},
	ResponseBody: capture.Rule{Enabled: true, MaxBytes: 4096, OnErrorOnly: true},
	RequestHeaders: capture.HeaderRule{
		Enabled: true,
		Exclude: []string{"Authorization", "Cookie"},
	},
	ResponseHeaders: capture.HeaderRule{Enabled: true, Mask: []string{"Set-Cookie"}},
}))
```

Headers land as `http.request.header.<lowercased-name>` /
`http.response.header.<lowercased-name>`. `httpservercore.WithYAMLConfig(cfg
core.Config)` applies the `integrations.httpserver` YAML section (same
`capture:` shape as [HTTP Client](http-client.md#capture-rules)); every
router adapter forwards it the same way it forwards any other
`httpservercore.Option`.

## See it running

- [`orders-api`](https://github.com/jhonsferg/argos/tree/main/samples/orders-api) - net/http
- [`catalog-service`](https://github.com/jhonsferg/argos/tree/main/samples/catalog-service) - gin
- [`notifications-worker`](https://github.com/jhonsferg/argos/tree/main/samples/notifications-worker) - chi
- [`shipments-service`](https://github.com/jhonsferg/argos/tree/main/samples/shipments-service) - echo
- [`reports-service`](https://github.com/jhonsferg/argos/tree/main/samples/reports-service) - fiber
- [`users-service`](https://github.com/jhonsferg/argos/tree/main/samples/users-service) - gorilla/mux
