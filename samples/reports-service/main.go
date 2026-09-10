// Command reports-service is a runnable Argos sample: a fiber REST API that
// pulls report files from an SFTP server and records their metadata in
// Cassandra. It demonstrates argosfiber (server middleware, fasthttp-based),
// argoscassandra (QueryObserver/BatchObserver), and argossftp (client
// wrapper) wired together under one argos.Init. See README.md for how to
// run it.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gocql/gocql"

	argos "github.com/jhonsferg/argos"
	argoscassandra "github.com/jhonsferg/argos/integrations/cassandra"
)

func main() {
	ctx := context.Background()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("reports-service"),
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

	cassandraHost := envOr("CASSANDRA_HOST", "localhost")
	keyspace := envOr("CASSANDRA_KEYSPACE", "reports")
	if err := ensureKeyspace(cassandraHost, keyspace); err != nil {
		log.Fatalf("ensureKeyspace: %v", err)
	}

	obs := argoscassandra.New()
	cluster := gocql.NewCluster(cassandraHost)
	cluster.Keyspace = keyspace
	cluster.ConnectTimeout = 20 * time.Second
	cluster.QueryObserver = obs
	cluster.BatchObserver = obs
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("gocql.CreateSession: %v", err)
	}
	defer session.Close()

	store := NewReportStore(session)
	if err := store.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	puller, closePuller, err := NewReportPuller(
		envOr("SFTP_ADDR", "localhost:2222"),
		envOr("SFTP_USER", "reports"),
		envOr("SFTP_PASSWORD", "reports"),
	)
	if err != nil {
		log.Fatalf("NewReportPuller: %v", err)
	}
	defer func() { _ = closePuller() }()

	api := NewAPI(puller, store, logger)

	addr := envOr("HTTP_ADDR", ":8084")
	logger.Info(ctx, "reports-service listening", argos.F("addr", addr))
	if err := api.Routes().Listen(addr); err != nil {
		log.Fatalf("fiber Listen: %v", err)
	}
}

// ensureKeyspace creates keyspace via a throwaway session with no keyspace
// set - gocql refuses to CreateSession with cluster.Keyspace pointed at a
// keyspace that doesn't exist yet, so this must run first, as a separate
// connection.
func ensureKeyspace(host, keyspace string) error {
	cluster := gocql.NewCluster(host)
	cluster.ConnectTimeout = 20 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return session.Query(
		`CREATE KEYSPACE IF NOT EXISTS ` + keyspace + ` WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`,
	).Exec()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
