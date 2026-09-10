package argosrabbitmq

import amqp "github.com/rabbitmq/amqp091-go"

// tableCarrier adapts amqp.Table (the type of both Publishing.Headers and
// Delivery.Headers) to propagation.TextMapCarrier. Callers must ensure the
// underlying table is non-nil before using a tableCarrier for injection
// (Set writes to it); reading a nil table is safe.
type tableCarrier struct{ table amqp.Table }

func (c tableCarrier) Get(key string) string {
	v, ok := c.table[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (c tableCarrier) Set(key, value string) {
	c.table[key] = value
}

func (c tableCarrier) Keys() []string {
	keys := make([]string, 0, len(c.table))
	for k := range c.table {
		keys = append(keys, k)
	}
	return keys
}
