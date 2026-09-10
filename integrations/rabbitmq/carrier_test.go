package argosrabbitmq

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestTableCarrier_RoundTrip(t *testing.T) {
	table := amqp.Table{}
	c := tableCarrier{table: table}

	c.Set("traceparent", "00-abc-def-01")
	if got := c.Get("traceparent"); got != "00-abc-def-01" {
		t.Errorf("Get = %q, want %q", got, "00-abc-def-01")
	}
	if got := c.Get("missing"); got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}

	keys := c.Keys()
	if len(keys) != 1 || keys[0] != "traceparent" {
		t.Errorf("Keys() = %v, want [traceparent]", keys)
	}
}

func TestTableCarrier_GetOnNilTableIsSafe(t *testing.T) {
	c := tableCarrier{}
	if got := c.Get("anything"); got != "" {
		t.Errorf("Get on nil table = %q, want empty", got)
	}
	if keys := c.Keys(); len(keys) != 0 {
		t.Errorf("Keys() on nil table = %v, want empty", keys)
	}
}

func TestTableCarrier_IgnoresNonStringValues(t *testing.T) {
	c := tableCarrier{table: amqp.Table{"count": 42}}
	if got := c.Get("count"); got != "" {
		t.Errorf("Get(count) = %q, want empty (non-string value)", got)
	}
}
