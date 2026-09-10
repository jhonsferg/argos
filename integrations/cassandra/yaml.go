package argoscassandra

import core "github.com/jhonsferg/argos"

// yamlConfig mirrors the subset of this module's Options that can be set
// from an argos YAML config file, under the "cassandra" key. Pointer
// fields distinguish "absent from YAML" from "explicitly set to the zero
// value", so an absent key never overrides an Option applied elsewhere in
// the chain.
type yamlConfig struct {
	QueryText       *bool `yaml:"query_text"`
	QueryTextMasked *bool `yaml:"query_text_masked"`
}

// WithYAMLConfig applies the "cassandra" section of cfg.Integrations (as
// loaded by argos.WithYAMLConfig or argos.FromYAML) to this module's
// Options. It is a no-op Option if the section is absent, so it is always
// safe to include even when the caller hasn't configured this module from
// YAML:
//
//	obs := argoscassandra.New(argoscassandra.WithYAMLConfig(cfg))
func WithYAMLConfig(cfg core.Config) Option {
	return func(c *config) {
		var y yamlConfig
		ok, err := core.DecodeIntegrationConfig(cfg, "cassandra", &y)
		if err != nil || !ok {
			return
		}
		if y.QueryText != nil {
			c.queryText = *y.QueryText
		}
		if y.QueryTextMasked != nil {
			c.queryTextMasked = *y.QueryTextMasked
		}
	}
}
