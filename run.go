package argos

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

// Run calls Init(ctx, opts...), then serve with a context cancelled on
// SIGINT/SIGTERM, then always calls Shutdown (bounded by the configured
// ShutdownTimeout, see WithShutdownTimeout, default 5s) once serve returns
// - regardless of whether serve returned an error or the process is
// shutting down because of a signal. It collapses the
// Init/defer-Shutdown/signal-handling boilerplate every long-running
// service using argos otherwise repeats:
//
//	err := argos.Run(context.Background(), []argos.Option{
//		argos.WithServiceName("orders-api"),
//	}, func(ctx context.Context, provider *argos.Provider) error {
//		srv := &http.Server{Addr: ":8080", Handler: handler}
//		go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
//		return srv.ListenAndServe()
//	})
//
// If Init fails, Run returns that error directly and never calls serve.
// The returned error is errors.Join(serve's error, Shutdown's error) -
// either may be nil, and a nil serve error with a non-nil Shutdown error
// (or vice versa) is still surfaced.
func Run(ctx context.Context, opts []Option, serve func(context.Context, *Provider) error) error {
	provider, err := Init(ctx, opts...)
	if err != nil {
		return err
	}

	runCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := serve(runCtx, provider)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), provider.shutdownTimeout)
	defer cancel()
	return errors.Join(serveErr, provider.Shutdown(shutdownCtx))
}
