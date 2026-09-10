// Command inventory-worker is the consumer half of the inventory-api /
// inventory-worker pair: it consumes the "inventory.stock.changed" Kafka
// topic inventory-api publishes to, and records each event in both
// MongoDB (argosmongo, an audit trail) and Cassandra (argoscassandra, an
// append-only analytics log). Because argoskafka.Consume extracts the
// trace context inventory-api propagated in the message headers, its
// consumer span - and the Mongo/Cassandra spans below it - continue the
// same trace as the HTTP request that produced the event, even though this
// is a wholly separate process. See ../inventory-api/README.md for the
// producer side and how to view the resulting trace in Grafana.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/gocql/gocql"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	argos "github.com/jhonsferg/argos"
	argoscassandra "github.com/jhonsferg/argos/integrations/cassandra"
	argosmongo "github.com/jhonsferg/argos/integrations/mongo"
)

func main() {
	err := argos.Run(context.Background(), []argos.Option{
		argos.WithServiceName("inventory-worker"),
		argos.WithServiceVersion("0.1.0"),
		argos.WithEnvironment("local"),
	}, run)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, provider *argos.Provider) error {
	logger := provider.Logger()

	mongoClient, err := mongo.Connect(options.Client().
		ApplyURI(envOr("MONGO_URI", "mongodb://localhost:27017")).
		SetMonitor(argosmongo.NewMonitor()))
	if err != nil {
		return err
	}
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()
	audit := NewAuditStore(mongoClient.Database("inventory"))

	cassandraHost := envOr("CASSANDRA_HOST", "localhost")
	keyspace := envOr("CASSANDRA_KEYSPACE", "inventory")
	if err := ensureKeyspace(cassandraHost, keyspace); err != nil {
		return err
	}
	obs := argoscassandra.New()
	cluster := gocql.NewCluster(cassandraHost)
	cluster.Keyspace = keyspace
	cluster.ConnectTimeout = 20 * time.Second
	cluster.QueryObserver = obs
	cluster.BatchObserver = obs
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}
	defer session.Close()
	analytics := NewAnalyticsStore(session)
	if err := analytics.Migrate(); err != nil {
		return err
	}

	kafkaCfg := sarama.NewConfig()
	kafkaCfg.Version = sarama.V2_8_0_0
	kafkaCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	group, err := sarama.NewConsumerGroup([]string{envOr("KAFKA_BROKER", "localhost:9092")}, envOr("KAFKA_GROUP", "inventory-worker"), kafkaCfg)
	if err != nil {
		return err
	}
	defer func() { _ = group.Close() }()

	processor := NewEventProcessor(audit, analytics)
	handler := consumerGroupHandler{processor: processor}

	logger.Info(ctx, "inventory-worker consuming", argos.F("topic", StockChangedTopic))
	for {
		if err := group.Consume(ctx, []string{StockChangedTopic}, handler); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			logger.Error(ctx, "consumer group session failed", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

// ensureKeyspace creates keyspace via a throwaway session with no keyspace
// set, same as reports-service does - gocql refuses to CreateSession with
// cluster.Keyspace pointed at a keyspace that doesn't exist yet.
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
