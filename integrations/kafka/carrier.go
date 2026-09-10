package argoskafka

import "github.com/IBM/sarama"

// producerCarrier adapts a *[]sarama.RecordHeader - the type of
// sarama.ProducerMessage.Headers - to propagation.TextMapCarrier, so
// otel.GetTextMapPropagator().Inject can write directly into it.
type producerCarrier struct{ headers *[]sarama.RecordHeader }

func (c producerCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c producerCarrier) Set(key, value string) {
	for i, h := range *c.headers {
		if string(h.Key) == key {
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
}

func (c producerCarrier) Keys() []string {
	keys := make([]string, len(*c.headers))
	for i, h := range *c.headers {
		keys[i] = string(h.Key)
	}
	return keys
}

// consumerCarrier adapts []*sarama.RecordHeader - the type of
// sarama.ConsumerMessage.Headers - to propagation.TextMapCarrier. Set is a
// no-op: a consumed message's headers aren't rewritten.
type consumerCarrier struct{ headers []*sarama.RecordHeader }

func (c consumerCarrier) Get(key string) string {
	for _, h := range c.headers {
		if h != nil && string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c consumerCarrier) Set(string, string) {}

func (c consumerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for _, h := range c.headers {
		if h != nil {
			keys = append(keys, string(h.Key))
		}
	}
	return keys
}
