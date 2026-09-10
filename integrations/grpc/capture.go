package argosgrpc

import (
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// attachMessageBody attaches msg (marshaled via protojson, when it
// implements proto.Message) to span under attrName, truncated to maxBytes
// when maxBytes > 0 - maxBytes <= 0 means "no limit", the same convention
// WithCapture's caller opts into by leaving it unset. Non-proto messages
// (shouldn't happen for a real gRPC call, but defensive) are silently
// skipped rather than erroring, matching this repo's "opt-in diagnostic,
// never fail the call over it" stance.
func attachMessageBody(span trace.Span, attrName string, msg any, maxBytes int) {
	pm, ok := msg.(proto.Message)
	if !ok {
		return
	}
	data, err := protojson.Marshal(pm)
	if err != nil {
		return
	}
	if maxBytes > 0 && len(data) > maxBytes {
		data = data[:maxBytes]
	}
	if len(data) > 0 {
		span.SetAttributes(attribute.String(attrName, string(data)))
	}
}

// mergeMetadata combines a response's header and trailer metadata into one
// map for ApplyHeaderRule - a real gRPC response can carry values in
// either, and callers of this package's client interceptor don't
// distinguish between them for capture purposes.
func mergeMetadata(header, trailer metadata.MD) map[string][]string {
	merged := make(map[string][]string, len(header)+len(trailer))
	for k, v := range header {
		merged[k] = v
	}
	for k, v := range trailer {
		merged[k] = v
	}
	return merged
}
