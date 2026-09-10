package main

import "fmt"

// Rule maps one or more vendor import paths (any one of which indicates the
// project uses that system) to the Argos integration that instruments it.
type Rule struct {
	Name        string
	Vendor      []string
	ArgosModule string
	Hint        string
}

// Suggestion is a Rule the project's imports matched without the
// corresponding Argos integration already present.
type Suggestion struct {
	Rule Rule
}

func (s Suggestion) String() string {
	return fmt.Sprintf("[suggest] %s detected - add %s\n           %s", s.Rule.Name, s.Rule.ArgosModule, s.Rule.Hint)
}

const coreModule = "github.com/jhonsferg/argos"

var coreRule = Rule{
	Name:        "Argos core",
	ArgosModule: coreModule,
	Hint:        "call argos.Init() once at startup before adding any integration.",
}

var httpClientRule = Rule{
	Name:        "outgoing HTTP client",
	Vendor:      []string{"net/http"},
	ArgosModule: "github.com/jhonsferg/argos/integrations/httpclient",
	Hint:        "wrap your http.Client's Transport with argoshttpclient.Wrap().",
}

var nethttpServerRule = Rule{
	Name:        "net/http server",
	Vendor:      []string{"net/http"},
	ArgosModule: "github.com/jhonsferg/argos/integrations/httpserver/nethttp",
	Hint:        "wrap your http.Handler with argosnethttp's middleware.",
}

// httpRouterRules are checked as a group: if the project imports one of
// these frameworks, net/http itself is just the transport underneath it,
// so the framework-specific rule takes priority over nethttpServerRule.
var httpRouterRules = []Rule{
	{
		Name:        "gin",
		Vendor:      []string{"github.com/gin-gonic/gin"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/httpserver/gin",
		Hint:        "wrap your gin.Engine with argosgin's middleware.",
	},
	{
		Name:        "echo",
		Vendor:      []string{"github.com/labstack/echo/v4"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/httpserver/echo",
		Hint:        "wrap your echo.Echo with argosecho's middleware.",
	},
	{
		Name:        "fiber",
		Vendor:      []string{"github.com/gofiber/fiber/v2"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/httpserver/fiber",
		Hint:        "wrap your fiber.App with argosfiber's middleware.",
	},
	{
		Name:        "chi",
		Vendor:      []string{"github.com/go-chi/chi/v5"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/httpserver/chi",
		Hint:        "wrap your chi.Router with argoschi's middleware.",
	},
	{
		Name:        "gorilla/mux",
		Vendor:      []string{"github.com/gorilla/mux"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/httpserver/gorillamux",
		Hint:        "wrap your mux.Router with argosgorillamux's middleware.",
	},
}

// dataAndMessagingRules are straightforward one-to-one checks: vendor
// import present, matching Argos module absent.
var dataAndMessagingRules = []Rule{
	{
		Name:        "database/sql",
		Vendor:      []string{"database/sql"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/sql",
		Hint:        "register argossql's driver wrapper instead of your driver directly.",
	},
	{
		Name:        "gorm",
		Vendor:      []string{"gorm.io/gorm"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/gorm",
		Hint:        "open your gorm.DB with argosgorm's plugin.",
	},
	{
		Name:        "MongoDB driver",
		Vendor:      []string{"go.mongodb.org/mongo-driver", "go.mongodb.org/mongo-driver/v2"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/mongo",
		Hint:        "attach argosmongo's CommandMonitor to your mongo.Client options.",
	},
	{
		Name:        "go-redis",
		Vendor:      []string{"github.com/redis/go-redis/v9"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/redis",
		Hint:        "add argosredis's hook to your redis.Client.",
	},
	{
		Name:        "gocql (Cassandra)",
		Vendor:      []string{"github.com/gocql/gocql"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/cassandra",
		Hint:        "attach argoscassandra's QueryObserver to your gocql.ClusterConfig.",
	},
	{
		Name:        "sarama (Kafka)",
		Vendor:      []string{"github.com/IBM/sarama"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/kafka",
		Hint:        "wrap your sarama producer/consumer calls with argoskafka.",
	},
	{
		Name:        "amqp091-go (RabbitMQ)",
		Vendor:      []string{"github.com/rabbitmq/amqp091-go"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/rabbitmq",
		Hint:        "wrap your amqp091.Channel publish/consume calls with argosrabbitmq.",
	},
	{
		Name:        "Azure Service Bus",
		Vendor:      []string{"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/azuresb",
		Hint:        "wrap your azservicebus Sender/Receiver calls with argosazuresb.",
	},
	{
		Name:        "GCP Pub/Sub",
		Vendor:      []string{"cloud.google.com/go/pubsub", "cloud.google.com/go/pubsub/v2"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/gcppubsub",
		Hint:        "wrap your pubsub publish/receive calls with argosgcppubsub.",
	},
	{
		Name:        "gRPC",
		Vendor:      []string{"google.golang.org/grpc"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/grpc",
		Hint:        "add argosgrpc's interceptors to your grpc.Server/ClientConn.",
	},
	{
		Name:        "pkg/sftp",
		Vendor:      []string{"github.com/pkg/sftp"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/sftp",
		Hint:        "wrap your *sftp.Client with argossftp.Wrap().",
	},
	{
		Name:        "net/smtp",
		Vendor:      []string{"net/smtp"},
		ArgosModule: "github.com/jhonsferg/argos/integrations/smtp",
		Hint:        "call argossmtp.SendMail instead of net/smtp.SendMail directly.",
	},
}

func hasAny(imports map[string]bool, paths []string) bool {
	for _, p := range paths {
		if imports[p] {
			return true
		}
	}
	return false
}

func missing(imports map[string]bool, r Rule) bool {
	return hasAny(imports, r.Vendor) && !imports[r.ArgosModule]
}

// Suggest evaluates every rule against the given set of import paths
// (as returned by CollectImports) and returns one Suggestion per vendor
// system detected without its matching Argos integration already imported.
// It never touches the filesystem or the network - it is pure so the rule
// table can be tested exhaustively with synthetic import sets.
func Suggest(imports map[string]bool) []Suggestion {
	var out []Suggestion

	if !imports[coreModule] {
		out = append(out, Suggestion{coreRule})
	}

	usesOtherRouter := false
	for _, r := range httpRouterRules {
		if hasAny(imports, r.Vendor) {
			usesOtherRouter = true
			if missing(imports, r) {
				out = append(out, Suggestion{r})
			}
		}
	}
	if !usesOtherRouter && missing(imports, nethttpServerRule) {
		out = append(out, Suggestion{nethttpServerRule})
	}

	if missing(imports, httpClientRule) {
		out = append(out, Suggestion{httpClientRule})
	}

	for _, r := range dataAndMessagingRules {
		if missing(imports, r) {
			out = append(out, Suggestion{r})
		}
	}

	return out
}
