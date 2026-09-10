package argosgrpc

import "google.golang.org/grpc/metadata"

// mdCarrier adapts metadata.MD to propagation.TextMapCarrier.
type mdCarrier metadata.MD

func (c mdCarrier) Get(key string) string {
	vals := metadata.MD(c).Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (c mdCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

func (c mdCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
