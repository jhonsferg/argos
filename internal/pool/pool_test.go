package pool

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestGetPutRoundTrip(t *testing.T) {
	s := GetAttrs()
	if len(*s) != 0 {
		t.Fatalf("expected zero-length slice, got len=%d", len(*s))
	}
	*s = append(*s, attribute.String("k", "v"))
	PutAttrs(s)

	s2 := GetAttrs()
	if len(*s2) != 0 {
		t.Fatalf("expected reset slice on reuse, got len=%d", len(*s2))
	}
	PutAttrs(s2)
}

func TestGetPutZeroAllocs(t *testing.T) {
	// Warm up so the pool has at least one slice of sufficient capacity.
	warm := GetAttrs()
	*warm = append(*warm, attribute.String("k", "v"))
	PutAttrs(warm)

	allocs := testing.AllocsPerRun(1000, func() {
		s := GetAttrs()
		*s = append(*s, attribute.String("k", "v"))
		PutAttrs(s)
	})
	if allocs != 0 {
		t.Fatalf("expected 0 allocs/op on warmed-up pool, got %v", allocs)
	}
}
