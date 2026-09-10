package argosgrpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jhonsferg/argos/capture"
	coremiddleware "github.com/jhonsferg/argos/middleware"
)

// UnaryServerInterceptor extracts any propagated trace context from
// incoming gRPC metadata, records a SERVER span/metric per call, and
// recovers panics in the handler (via core/middleware.Handle) into a
// codes.Internal status instead of crashing the server.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	cfg := newConfig("rpc.server.duration", opts...)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		ctx, span, start := cfg.startServer(ctx, info.FullMethod)
		defer func() {
			if panicErr := coremiddleware.Handle(ctx, cfg.logger, recover()); panicErr != nil {
				err = status.Error(codes.Internal, panicErr.Error())
			}
			cfg.end(ctx, span, info.FullMethod, err, start)
		}()
		return handler(ctx, req)
	}
}

// StreamServerInterceptor is UnaryServerInterceptor for streaming RPCs: the
// handler sees a wrapped ServerStream whose Context carries the extracted
// trace context.
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	cfg := newConfig("rpc.server.duration", opts...)
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		ctx, span, start := cfg.startServer(ss.Context(), info.FullMethod)
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}
		defer func() {
			if panicErr := coremiddleware.Handle(ctx, cfg.logger, recover()); panicErr != nil {
				err = status.Error(codes.Internal, panicErr.Error())
			}
			cfg.end(ctx, span, info.FullMethod, err, start)
		}()
		return handler(srv, wrapped)
	}
}

// UnaryClientInterceptor injects the active trace context into outgoing
// gRPC metadata and records a CLIENT span/metric per call. When configured
// via WithCapture, it also attaches request/response message bodies and
// metadata to the span - see capture.go for the exact attribute names.
func UnaryClientInterceptor(opts ...Option) grpc.UnaryClientInterceptor {
	cfg := newConfig("rpc.client.duration", opts...)
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
		ctx, span, start := cfg.startClient(ctx, method)

		var respHeader, respTrailer metadata.MD
		if cfg.rules.ResponseHeaders.Enabled {
			callOpts = append(callOpts, grpc.Header(&respHeader), grpc.Trailer(&respTrailer))
		}

		err := invoker(ctx, method, req, reply, cc, callOpts...)
		isError := err != nil

		if cfg.rules.RequestBody.Enabled && (!cfg.rules.RequestBody.OnErrorOnly || isError) {
			attachMessageBody(span, "argos.rpc.request.body", req, cfg.rules.RequestBody.MaxBytes)
		}
		if cfg.rules.ResponseBody.Enabled && (!cfg.rules.ResponseBody.OnErrorOnly || isError) {
			attachMessageBody(span, "argos.rpc.response.body", reply, cfg.rules.ResponseBody.MaxBytes)
		}
		if outMD, ok := metadata.FromOutgoingContext(ctx); ok {
			capture.ApplyHeaderRule(span, "rpc.grpc.request.metadata.", map[string][]string(outMD), cfg.rules.RequestHeaders, isError)
		}
		capture.ApplyHeaderRule(span, "rpc.grpc.response.metadata.", mergeMetadata(respHeader, respTrailer), cfg.rules.ResponseHeaders, isError)

		cfg.end(ctx, span, method, err, start)
		return err
	}
}

// StreamClientInterceptor is UnaryClientInterceptor for streaming RPCs. The
// span it records covers only stream creation, not the lifetime of the
// stream or its final status - gRPC's client streamer has no single point
// where the whole call's outcome is known, so this matches every other
// client-side stream interceptor's usual scope.
func StreamClientInterceptor(opts ...Option) grpc.StreamClientInterceptor {
	cfg := newConfig("rpc.client.duration", opts...)
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx, span, start := cfg.startClient(ctx, method)
		cs, err := streamer(ctx, desc, cc, method, callOpts...)
		cfg.end(ctx, span, method, err, start)
		return cs, err
	}
}

// wrappedServerStream overrides ServerStream.Context so a streaming
// handler observes the trace context extracted from incoming metadata.
// grpc-go has no exported helper for this - every interceptor library
// (including grpc-go's own contrib packages) defines this same small type.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context { return w.ctx }
