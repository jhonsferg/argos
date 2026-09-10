package main

import "testing"

func names(suggestions []Suggestion) map[string]bool {
	m := make(map[string]bool, len(suggestions))
	for _, s := range suggestions {
		m[s.Rule.Name] = true
	}
	return m
}

func TestSuggest_EmptyProjectSuggestsOnlyCore(t *testing.T) {
	got := names(Suggest(map[string]bool{}))
	if len(got) != 1 || !got["Argos core"] {
		t.Fatalf("suggestions = %v, want only {Argos core}", got)
	}
}

func TestSuggest_CoreImportedNoOtherSuggestions(t *testing.T) {
	got := Suggest(map[string]bool{coreModule: true})
	if len(got) != 0 {
		t.Fatalf("expected no suggestions once core is imported, got %v", names(got))
	}
}

func TestSuggest_RouterDetectedWithoutArgosModule(t *testing.T) {
	imports := map[string]bool{
		coreModule:                 true,
		"github.com/gin-gonic/gin": true,
	}
	got := names(Suggest(imports))
	if !got["gin"] {
		t.Errorf("expected a gin suggestion, got %v", got)
	}
	if got["net/http server"] {
		t.Errorf("net/http server should not be suggested when a router is already in use, got %v", got)
	}
}

func TestSuggest_RouterAlreadyWiredNoSuggestion(t *testing.T) {
	imports := map[string]bool{
		coreModule:                 true,
		"github.com/gin-gonic/gin": true,
		"github.com/jhonsferg/argos/integrations/httpserver/gin": true,
	}
	got := names(Suggest(imports))
	if got["gin"] {
		t.Errorf("gin should not be suggested once its Argos module is imported, got %v", got)
	}
}

func TestSuggest_BareNetHTTPSuggestsNethttpServer(t *testing.T) {
	imports := map[string]bool{
		coreModule: true,
		"net/http": true,
	}
	got := names(Suggest(imports))
	if !got["net/http server"] {
		t.Errorf("expected a net/http server suggestion, got %v", got)
	}
	if !got["outgoing HTTP client"] {
		t.Errorf("expected an outgoing HTTP client suggestion, got %v", got)
	}
}

func TestSuggest_DataAndMessagingRules(t *testing.T) {
	tests := []struct {
		name        string
		vendor      string
		argosModule string
	}{
		{"database/sql", "database/sql", "github.com/jhonsferg/argos/integrations/sql"},
		{"gorm", "gorm.io/gorm", "github.com/jhonsferg/argos/integrations/gorm"},
		{"MongoDB driver", "go.mongodb.org/mongo-driver/v2", "github.com/jhonsferg/argos/integrations/mongo"},
		{"go-redis", "github.com/redis/go-redis/v9", "github.com/jhonsferg/argos/integrations/redis"},
		{"gocql (Cassandra)", "github.com/gocql/gocql", "github.com/jhonsferg/argos/integrations/cassandra"},
		{"sarama (Kafka)", "github.com/IBM/sarama", "github.com/jhonsferg/argos/integrations/kafka"},
		{"amqp091-go (RabbitMQ)", "github.com/rabbitmq/amqp091-go", "github.com/jhonsferg/argos/integrations/rabbitmq"},
		{"Azure Service Bus", "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus", "github.com/jhonsferg/argos/integrations/azuresb"},
		{"GCP Pub/Sub", "cloud.google.com/go/pubsub/v2", "github.com/jhonsferg/argos/integrations/gcppubsub"},
		{"gRPC", "google.golang.org/grpc", "github.com/jhonsferg/argos/integrations/grpc"},
		{"pkg/sftp", "github.com/pkg/sftp", "github.com/jhonsferg/argos/integrations/sftp"},
		{"net/smtp", "net/smtp", "github.com/jhonsferg/argos/integrations/smtp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missingImports := map[string]bool{coreModule: true, tt.vendor: true}
			got := names(Suggest(missingImports))
			if !got[tt.name] {
				t.Errorf("expected a %q suggestion with imports %v, got %v", tt.name, missingImports, got)
			}

			wiredImports := map[string]bool{coreModule: true, tt.vendor: true, tt.argosModule: true}
			got = names(Suggest(wiredImports))
			if got[tt.name] {
				t.Errorf("did not expect a %q suggestion once its Argos module is imported, got %v", tt.name, got)
			}
		})
	}
}

func TestSuggest_FullyWiredProjectHasNoSuggestions(t *testing.T) {
	imports := map[string]bool{
		coreModule:                 true,
		"github.com/gin-gonic/gin": true,
		"github.com/jhonsferg/argos/integrations/httpserver/gin": true,
		"net/http": true,
		"github.com/jhonsferg/argos/integrations/httpclient": true,
		"github.com/redis/go-redis/v9":                       true,
		"github.com/jhonsferg/argos/integrations/redis":      true,
	}
	got := Suggest(imports)
	if len(got) != 0 {
		t.Fatalf("expected no suggestions, got %v", names(got))
	}
}
