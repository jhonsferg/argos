package argosazuresb

// propertiesCarrier adapts a map[string]any - the type of both
// Message.ApplicationProperties and ReceivedMessage.ApplicationProperties -
// to propagation.TextMapCarrier. Callers must ensure the underlying map is
// non-nil before using a propertiesCarrier for injection (Set writes to
// it); reading a nil map is safe.
type propertiesCarrier struct{ props map[string]any }

func (c propertiesCarrier) Get(key string) string {
	v, ok := c.props[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (c propertiesCarrier) Set(key, value string) {
	c.props[key] = value
}

func (c propertiesCarrier) Keys() []string {
	keys := make([]string, 0, len(c.props))
	for k := range c.props {
		keys = append(keys, k)
	}
	return keys
}
