package httpservercore

import (
	core "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/capture"
)

// yamlConfig mirrors capture.HTTPRules for the "httpserver" YAML section,
// under a "capture:" key - see integrations/httpclient/yaml.go for the
// same shape on the outbound side.
type yamlConfig struct {
	Capture capture.YAMLHTTPRules `yaml:"capture"`
}

// WithYAMLConfig applies the "httpserver" section of cfg.Integrations (as
// loaded by argos.WithYAMLConfig or argos.FromYAML) to this Instrumentor's
// capture rules. It is a no-op Option if the section is absent, so it is
// always safe to include even when the caller hasn't configured this
// module from YAML:
//
//	argosnethttp.Middleware(httpservercore.WithYAMLConfig(cfg))
func WithYAMLConfig(cfg core.Config) Option {
	return func(i *Instrumentor) {
		var y yamlConfig
		ok, err := core.DecodeIntegrationConfig(cfg, "httpserver", &y)
		if err != nil || !ok {
			return
		}
		y.Capture.Apply(&i.rules)
	}
}
