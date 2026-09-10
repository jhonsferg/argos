package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	argoschi "github.com/jhonsferg/argos/integrations/httpserver/chi"
)

// Routes builds the small chi-based admin API: this worker's only HTTP
// surface is a health check, since its real work happens off the RabbitMQ
// queue (see worker.go).
func Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(argoschi.Middleware())
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return r
}
