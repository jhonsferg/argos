package argoshttpclient

import (
	core "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/capture"
)

// yamlConfig mirrors capture.HTTPRules for the "httpclient" YAML section,
// under a "capture:" key:
//
//	integrations:
//	  httpclient:
//	    capture:
//	      request_headers: { enabled: true, exclude: ["Authorization"] }
type yamlConfig struct {
	Capture capture.YAMLHTTPRules `yaml:"capture"`
}

// WithYAMLConfig applies the "httpclient" section of cfg.Integrations (as
// loaded by argos.WithYAMLConfig or argos.FromYAML) to this module's
// capture rules. It is a no-op Option if the section is absent, so it is
// always safe to include even when the caller hasn't configured this
// module from YAML:
//
//	client := argoshttpclient.NewClient(http.DefaultClient, argoshttpclient.WithYAMLConfig(cfg))
func WithYAMLConfig(cfg core.Config) Option {
	return func(rt *roundTripper) {
		var y yamlConfig
		ok, err := core.DecodeIntegrationConfig(cfg, "httpclient", &y)
		if err != nil || !ok {
			return
		}
		y.Capture.Apply(&rt.rules)
	}
}
