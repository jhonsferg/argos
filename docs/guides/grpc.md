# gRPC

`integrations/grpc` provides standard unary/stream server and client
interceptors, plus a `metadata.MD` carrier so trace context propagates from
a client call to the server handling it - the same guarantee every other
integration in this library gives for cross-process calls.

```go
import argosgrpc "github.com/jhonsferg/argos/integrations/grpc"

// server
srv := grpc.NewServer(grpc.UnaryInterceptor(argosgrpc.UnaryServerInterceptor()))

// client
conn, _ := grpc.NewClient(addr,
	grpc.WithTransportCredentials(insecure.NewCredentials()),
	grpc.WithUnaryInterceptor(argosgrpc.UnaryClientInterceptor()),
)
```

Streaming interceptors are named the same way:
`StreamServerInterceptor()`/`StreamClientInterceptor()`.

## Capture rules

`UnaryClientInterceptor` accepts `WithCapture(rules capture.HTTPRules)` to
capture outgoing calls' request/response messages and metadata - the same
`capture.HTTPRules` shape used by [HTTP Client](http-client.md#capture-rules)
and [HTTP Servers](http-servers.md#capture-rules), with metadata standing in
for headers and the marshaled message (via `protojson`, for any message
implementing `proto.Message`) standing in for the body. Off by default;
server-side interceptors don't use this option.

```go
import "github.com/jhonsferg/argos/capture"

conn, _ := grpc.NewClient(addr,
	grpc.WithTransportCredentials(insecure.NewCredentials()),
	grpc.WithUnaryInterceptor(argosgrpc.UnaryClientInterceptor(argosgrpc.WithCapture(capture.HTTPRules{
		RequestBody:  capture.Rule{Enabled: true, MaxBytes: 4096},
		ResponseBody: capture.Rule{Enabled: true, MaxBytes: 4096},
		RequestHeaders: capture.HeaderRule{
			Enabled: true,
			Exclude: []string{"authorization"},
		},
		ResponseHeaders: capture.HeaderRule{Enabled: true},
	}))),
)
```

Bodies land as `argos.rpc.request.body`/`argos.rpc.response.body` (no OTel
semconv equivalent exists for gRPC message bodies); metadata lands as
`rpc.grpc.request.metadata.<key>`/`rpc.grpc.response.metadata.<key>` per
entry. Response metadata is read from both the call's header and trailer.

`WithYAMLConfig(cfg core.Config)` applies the `integrations.grpc` section
(same `capture:` shape as [HTTP Client](http-client.md#capture-rules)).

## See it running

[`users-service`](https://github.com/jhonsferg/argos/tree/main/samples/users-service)
is a gorilla/mux HTTP gateway calling a gRPC backend - one trace spans both
processes, provable end to end with nothing but `go run` (no external
infrastructure needed).
