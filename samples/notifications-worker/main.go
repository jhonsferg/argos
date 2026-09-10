// Command notifications-worker is a runnable Argos sample: a background
// worker that consumes a RabbitMQ queue and sends an email per message via
// SMTP, with a small chi-based admin API exposing only /healthz. It
// demonstrates argoschi (server middleware), argosrabbitmq (trace-aware
// consumer wrapper), and argossmtp (mail sender) wired together under one
// argos.Init. See README.md for how to run it.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	argos "github.com/jhonsferg/argos"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("notifications-worker"),
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
	logger := provider.Logger()

	conn, err := amqp.Dial(envOr("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"))
	if err != nil {
		log.Fatalf("amqp.Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("conn.Channel: %v", err)
	}
	defer func() { _ = channel.Close() }()

	const queueName = "notifications"
	if _, err := channel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		log.Fatalf("QueueDeclare: %v", err)
	}

	mailer := NewSMTPMailer(envOr("SMTP_ADDR", "localhost:1025"), "notifications@example.com")
	worker := NewWorker(channel, mailer, logger)

	go func() {
		if err := worker.Run(ctx, queueName); err != nil {
			logger.Error(ctx, "worker stopped with error", err)
		}
	}()

	addr := envOr("HTTP_ADDR", ":8082")
	srv := &http.Server{Addr: addr, Handler: Routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info(ctx, "notifications-worker listening", argos.F("addr", addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
