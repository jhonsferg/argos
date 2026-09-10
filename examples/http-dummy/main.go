// Command http-dummy proves the F1 HTTP instrumentation end-to-end in one
// process: a net/http server instrumented with argosnethttp, called by an
// argoshttpclient-instrumented client, both under one argos-core.Init.
package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	argos "github.com/jhonsferg/argos"
	argoshttpclient "github.com/jhonsferg/argos/integrations/httpclient"
	argosnethttp "github.com/jhonsferg/argos/integrations/httpserver/nethttp"
)

func main() {
	ctx := context.Background()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("http-dummy"),
		argos.WithServiceVersion("0.1.0"),
		argos.WithEnvironment("local"),
	)
	if err != nil {
		log.Fatalf("argos.Init: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			log.Printf("argos.Shutdown: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		provider.Logger().Info(r.Context(), "handling health check")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// httptest.NewServer keeps this example a single runnable process (a
	// real listener, no manual goroutine/shutdown bookkeeping) while still
	// exercising the real net/http server path end-to-end.
	srv := httptest.NewServer(argosnethttp.Middleware()(mux))
	defer srv.Close()

	client := argoshttpclient.NewClient(srv.Client())
	resp, err := client.Get(srv.URL + "/health")
	if err != nil {
		log.Fatalf("client.Get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("read body: %v", err)
	}
	provider.Logger().Info(ctx, "received response", argos.F("status", resp.StatusCode), argos.F("body", string(body)))
}
