// Command shipments-service is a runnable Argos sample: an echo REST API
// backed by MongoDB, publishing a GCP Pub/Sub event whenever a shipment's
// status is updated. It demonstrates argosecho (server middleware),
// argosmongo (CommandMonitor), and argosgcppubsub (publisher wrapper) wired
// together under one argos.Init. See README.md for how to run it.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	argos "github.com/jhonsferg/argos"
	argosmongo "github.com/jhonsferg/argos/integrations/mongo"
)

func main() {
	ctx := context.Background()

	provider, err := argos.Init(ctx,
		argos.WithServiceName("shipments-service"),
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

	mongoClient, err := mongo.Connect(options.Client().
		ApplyURI(envOr("MONGO_URI", "mongodb://localhost:27017")).
		SetMonitor(argosmongo.NewMonitor()))
	if err != nil {
		log.Fatalf("mongo.Connect: %v", err)
	}
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	store := NewShipmentStore(mongoClient.Database("shipments"))

	projectID := envOr("PUBSUB_PROJECT_ID", "shipments-local")
	pubsubClient, err := pubsub.NewClient(ctx, projectID,
		option.WithEndpoint(envOr("PUBSUB_EMULATOR_HOST", "localhost:8085")),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		log.Fatalf("pubsub.NewClient: %v", err)
	}
	defer func() { _ = pubsubClient.Close() }()

	topicName := fmt.Sprintf("projects/%s/topics/shipment.updated", projectID)
	if err := ensureTopic(ctx, pubsubClient, topicName); err != nil {
		log.Fatalf("ensureTopic: %v", err)
	}
	publisher := pubsubClient.Publisher(topicName)
	defer publisher.Stop()

	api := NewAPI(store, NewEventPublisher(publisher), logger)

	addr := envOr("HTTP_ADDR", ":8083")
	logger.Info(ctx, "shipments-service listening", argos.F("addr", addr))
	srv := &http.Server{Addr: addr, Handler: api.Routes(), ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

// ensureTopic creates topicName, tolerating it already existing so the
// sample can be restarted against the same emulator without a fresh
// docker-compose up.
func ensureTopic(ctx context.Context, client *pubsub.Client, topicName string) error {
	_, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topicName})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return err
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
