// Package pool provides sync.Pool-backed reuse of hot-path allocations shared
// across argos-core and its integrations.
package pool

import (
	"sync"

	"go.opentelemetry.io/otel/attribute"
)

// defaultAttrCap is the pre-allocated capacity for pooled attribute slices.
// Most spans/log records attach far fewer than this many attributes, so a
// slice sized to it rarely needs to grow (and re-allocate) on Put/Get reuse.
const defaultAttrCap = 8

var attrSlicePool = sync.Pool{
	New: func() any {
		s := make([]attribute.KeyValue, 0, defaultAttrCap)
		return &s
	},
}

// GetAttrs returns a zero-length []attribute.KeyValue with spare capacity
// from the pool. Callers must return it via PutAttrs when done.
func GetAttrs() *[]attribute.KeyValue {
	return attrSlicePool.Get().(*[]attribute.KeyValue)
}

// PutAttrs resets the slice to zero length and returns it to the pool. It
// must not be called with a slice still referenced elsewhere (e.g. after it
// was passed to a span that retains it).
func PutAttrs(s *[]attribute.KeyValue) {
	*s = (*s)[:0]
	attrSlicePool.Put(s)
}
