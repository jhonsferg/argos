package argos

import (
	"context"
	"errors"
	"testing"
)

func TestRun_InitFailure_ServeNeverCalled(t *testing.T) {
	called := false
	err := Run(context.Background(), []Option{
		WithYAMLConfig("does-not-exist.yaml"),
	}, func(ctx context.Context, provider *Provider) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected an error from a missing YAML config file")
	}
	if called {
		t.Error("serve must not be called when Init fails")
	}
}

func TestRun_ServeErrorIsSurfaced(t *testing.T) {
	wantErr := errors.New("serve boom")
	err := Run(context.Background(), []Option{
		WithServiceName("argos-run-test"),
		WithShutdownTimeout(0), // don't wait on the unreachable default collector
	}, func(ctx context.Context, provider *Provider) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestRun_ServeReceivesACancellableContext(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	cancel() // already done before Run even starts

	var sawDone bool
	err := Run(parentCtx, []Option{
		WithServiceName("argos-run-test"),
		WithShutdownTimeout(0),
	}, func(ctx context.Context, provider *Provider) error {
		select {
		case <-ctx.Done():
			sawDone = true
		default:
		}
		return nil
	})
	_ = err // Shutdown against the unreachable default collector may itself error; not asserted here.
	if !sawDone {
		t.Error("serve's context was not already Done despite the parent context being cancelled before Run")
	}
}

func TestRun_ProviderIsUsable(t *testing.T) {
	var loggedNil bool
	err := Run(context.Background(), []Option{
		WithServiceName("argos-run-test"),
		WithShutdownTimeout(0),
	}, func(ctx context.Context, provider *Provider) error {
		loggedNil = provider == nil
		return nil
	})
	_ = err
	if loggedNil {
		t.Error("serve was called with a nil Provider")
	}
}
