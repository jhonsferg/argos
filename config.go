package argos

import (
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
	"gopkg.in/yaml.v3"
)

// Protocol selects the wire protocol used for every OTLP exporter (traces,
// metrics, logs).
type Protocol string

const (
	ProtocolGRPC Protocol = "grpc"
	ProtocolHTTP Protocol = "http"
)

// Config holds everything Init needs. Build it with functional Options, or
// load defaults from the environment/a YAML file and layer Options on top.
type Config struct {
	ServiceName    string `yaml:"service_name"`
	ServiceVersion string `yaml:"service_version"`
	Environment    string `yaml:"environment"`

	Endpoint string   `yaml:"otlp_endpoint"`
	Protocol Protocol `yaml:"otlp_protocol"`
	Insecure bool     `yaml:"otlp_insecure"`

	// SampleRatio is used to build a trace.ParentBased(trace.TraceIDRatioBased)
	// sampler unless Sampler is set explicitly.
	SampleRatio float64       `yaml:"sample_ratio"`
	Sampler     trace.Sampler `yaml:"-"`
	Logger      Logger        `yaml:"-"`

	// ShutdownTimeout bounds how long Run waits for Shutdown to finish
	// after the serve callback returns. Only used by Run, not by Init
	// itself. Go-code-only for now (no YAML tag) - see WithShutdownTimeout.
	ShutdownTimeout time.Duration `yaml:"-"`

	// Integrations holds arbitrary per-integration YAML sections, keyed by
	// each integration's own name (e.g. "sql"). Config itself never
	// inspects the contents - see DecodeIntegrationConfig.
	Integrations map[string]yaml.Node `yaml:"integrations"`

	// err carries a failure from an Option (currently only WithYAMLConfig)
	// without changing the Option function signature. Init checks it once,
	// after every Option has run.
	err error
}

// Option mutates a Config during Init.
type Option func(*Config)

func WithServiceName(name string) Option {
	return func(c *Config) { c.ServiceName = name }
}

func WithServiceVersion(version string) Option {
	return func(c *Config) { c.ServiceVersion = version }
}

func WithEnvironment(env string) Option {
	return func(c *Config) { c.Environment = env }
}

func WithOTLPEndpoint(endpoint string) Option {
	return func(c *Config) { c.Endpoint = endpoint }
}

func WithOTLPProtocol(p Protocol) Option {
	return func(c *Config) { c.Protocol = p }
}

func WithOTLPInsecure(insecure bool) Option {
	return func(c *Config) { c.Insecure = insecure }
}

func WithSampleRatio(ratio float64) Option {
	return func(c *Config) { c.SampleRatio = ratio }
}

func WithSampler(s trace.Sampler) Option {
	return func(c *Config) { c.Sampler = s }
}

func WithLogger(l Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithShutdownTimeout bounds how long Run waits for Shutdown to finish
// after the serve callback returns. Default 5s; only observed by Run, not
// by a bare Init/Shutdown pair.
func WithShutdownTimeout(d time.Duration) Option {
	return func(c *Config) { c.ShutdownTimeout = d }
}

// WithYAMLConfig loads path and unmarshals it onto the Config as it stands
// at this point in the Option chain - unlike WithConfig (a wholesale
// replace), it layers on top of defaultConfig/fromEnv and any Option
// applied earlier, and any Option applied after it can still override a
// field it set. This is the single-call equivalent of:
//
//	cfg, err := argos.FromYAML(path)
//	provider, err := argos.Init(ctx, argos.WithConfig(cfg))
//
// A read or parse failure is reported by Init returning that error - it
// does not panic and does not stop later Options in the chain from running.
func WithYAMLConfig(path string) Option {
	return func(c *Config) {
		data, readErr := os.ReadFile(path) // #nosec G304 -- path is an operator-supplied startup config location, not attacker input
		if readErr != nil {
			c.err = fmt.Errorf("argos: read config %q: %w", path, readErr)
			return
		}
		if unmarshalErr := yaml.Unmarshal(data, c); unmarshalErr != nil {
			c.err = fmt.Errorf("argos: parse config %q: %w", path, unmarshalErr)
		}
	}
}

// DecodeIntegrationConfig decodes the YAML section registered under key (as
// loaded by WithYAMLConfig/FromYAML into Config.Integrations) into out. It
// returns ok=false, err=nil when no such section exists - callers keep
// whatever defaults/Options they already have in that case.
func DecodeIntegrationConfig(cfg Config, key string, out any) (ok bool, err error) {
	node, present := cfg.Integrations[key]
	if !present {
		return false, nil
	}
	if decodeErr := node.Decode(out); decodeErr != nil {
		return false, fmt.Errorf("argos: decode %q integration config: %w", key, decodeErr)
	}
	return true, nil
}

// defaultConfig returns the baseline Config before env/YAML/Options are
// applied.
func defaultConfig() Config {
	return Config{
		ServiceName:     "unknown_service",
		Endpoint:        "localhost:4317",
		Protocol:        ProtocolGRPC,
		Insecure:        true,
		SampleRatio:     1.0,
		ShutdownTimeout: 5 * time.Second,
	}
}

// fromEnv layers standard OTEL_* environment variables onto cfg. Recognized:
// OTEL_SERVICE_NAME, OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_PROTOCOL,
// OTEL_EXPORTER_OTLP_INSECURE, DEPLOYMENT_ENVIRONMENT.
func fromEnv(cfg *Config) {
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		cfg.ServiceName = v
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); v != "" {
		switch v {
		case "grpc":
			cfg.Protocol = ProtocolGRPC
		case "http/protobuf", "http":
			cfg.Protocol = ProtocolHTTP
		}
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"); v != "" {
		cfg.Insecure = v == "true"
	}
	if v := os.Getenv("DEPLOYMENT_ENVIRONMENT"); v != "" {
		cfg.Environment = v
	}
}

// FromYAML loads a Config from a YAML file, layered on top of defaultConfig().
// It does not read environment variables or apply Options - callers combine
// those explicitly:
//
//	cfg, err := argos.FromYAML("config.yaml")
//	provider, err := argos.Init(ctx, argos.WithConfig(cfg), argos.WithServiceVersion(v))
func FromYAML(path string) (Config, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(path) // #nosec G304 -- path is an operator-supplied startup config location, not attacker input
	if err != nil {
		return Config{}, fmt.Errorf("argos: read config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("argos: parse config %q: %w", path, err)
	}
	return cfg, nil
}

// WithConfig replaces the working Config wholesale - typically the result of
// FromYAML - before subsequent Options are applied.
func WithConfig(base Config) Option {
	return func(c *Config) { *c = base }
}
