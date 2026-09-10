package argosgrpc

import (
	core "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/capture"
)

// yamlConfig mirrors capture.HTTPRules for the "grpc" YAML section, under a
// "capture:" key:
//
//	integrations:
//	  grpc:
//	    capture:
//	      request_headers: { enabled: true, exclude: ["authorization"] }
type yamlConfig struct {
	Capture capture.YAMLHTTPRules `yaml:"capture"`
}

// WithYAMLConfig applies the "grpc" section of cfg.Integrations (as loaded
// by argos.WithYAMLConfig or argos.FromYAML) to this module's capture
// rules. It is a no-op Option if the section is absent, so it is always
// safe to include even when the caller hasn't configured this module from
// YAML:
//
//	client := argosgrpc.UnaryClientInterceptor(argosgrpc.WithYAMLConfig(cfg))
func WithYAMLConfig(cfg core.Config) Option {
	return func(c *config) {
		var y yamlConfig
		ok, err := core.DecodeIntegrationConfig(cfg, "grpc", &y)
		if err != nil || !ok {
			return
		}
		y.Capture.Apply(&c.rules)
	}
}
