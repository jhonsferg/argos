package argoscontrib_test

import (
	"context"
	"testing"

	argoscontrib "github.com/jhonsferg/argos/integrations/contrib"
)

// BenchmarkDoThing_Baseline measures the underlying WidgetClient directly -
// the "without Argos" comparison point.
func BenchmarkDoThing_Baseline(b *testing.B) {
	client := &fakeWidgetClient{}
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := client.DoThing(ctx, "widget-1"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDoThing_Instrumented measures the same operation through
// argoscontrib.Wrap, documenting the allocation cost of the span+metric
// pipeline this template adds per call.
func BenchmarkDoThing_Instrumented(b *testing.B) {
	client := argoscontrib.Wrap(&fakeWidgetClient{})
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := client.DoThing(ctx, "widget-1"); err != nil {
			b.Fatal(err)
		}
	}
}
