package argoscontrib_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argoscontrib "github.com/jhonsferg/argos/integrations/contrib"
)

func setTracer(t testing.TB) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return exp
}

// fakeWidgetClient is a hand-rolled WidgetClient for testing, standing in
// for whatever real SDK type argoscontrib.Wrap would normally receive.
type fakeWidgetClient struct {
	err error
}

func (f *fakeWidgetClient) DoThing(_ context.Context, _ string) error {
	return f.err
}

func TestDoThing_RecordsSpan(t *testing.T) {
	exp := setTracer(t)
	client := argoscontrib.Wrap(&fakeWidgetClient{})

	if err := client.DoThing(context.Background(), "widget-1"); err != nil {
		t.Fatalf("DoThing: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "DoThing" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "DoThing")
	}
	if spans[0].Status.Code == codes.Error {
		t.Errorf("expected an OK span, got error status: %s", spans[0].Status.Description)
	}
}

func TestDoThing_RecordsErrorFromNext(t *testing.T) {
	exp := setTracer(t)
	wantErr := errors.New("widget exploded")
	client := argoscontrib.Wrap(&fakeWidgetClient{err: wantErr})

	err := client.DoThing(context.Background(), "widget-1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("DoThing error = %v, want %v", err, wantErr)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}
