package argosfiber

import "github.com/valyala/fasthttp"

// requestHeaderCarrier adapts *fasthttp.RequestHeader to
// propagation.TextMapCarrier, since fiber runs on fasthttp rather than
// net/http and has no http.Header to reuse propagation.HeaderCarrier with.
type requestHeaderCarrier struct{ header *fasthttp.RequestHeader }

func (c requestHeaderCarrier) Get(key string) string {
	return string(c.header.Peek(key))
}

func (c requestHeaderCarrier) Set(key, value string) {
	c.header.Set(key, value)
}

func (c requestHeaderCarrier) Keys() []string {
	keys := make([]string, 0, c.header.Len())
	for k := range c.header.All() {
		keys = append(keys, string(k))
	}
	return keys
}
