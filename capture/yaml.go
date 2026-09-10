package capture

// YAMLRule mirrors Rule with pointer fields for YAML decoding, so an
// absent key never overrides an Option applied elsewhere in the chain.
// Shared by every integration that exposes HTTPRules via WithYAMLConfig
// (httpclient, httpserver/core, grpc) instead of each redefining it.
type YAMLRule struct {
	Enabled     *bool `yaml:"enabled"`
	MaxBytes    *int  `yaml:"max_bytes"`
	OnErrorOnly *bool `yaml:"on_error_only"`
}

// Apply overlays the YAML-set fields of y onto r. A nil receiver is a
// no-op, so a YAML section that omits this rule entirely leaves r
// untouched.
func (y *YAMLRule) Apply(r *Rule) {
	if y == nil {
		return
	}
	if y.Enabled != nil {
		r.Enabled = *y.Enabled
	}
	if y.MaxBytes != nil {
		r.MaxBytes = *y.MaxBytes
	}
	if y.OnErrorOnly != nil {
		r.OnErrorOnly = *y.OnErrorOnly
	}
}

// YAMLHeaderRule mirrors HeaderRule the same way.
type YAMLHeaderRule struct {
	Enabled     *bool    `yaml:"enabled"`
	OnErrorOnly *bool    `yaml:"on_error_only"`
	Exclude     []string `yaml:"exclude"`
	Mask        []string `yaml:"mask"`
}

// Apply overlays the YAML-set fields of y onto r. A nil receiver is a
// no-op. Exclude/Mask, when present in YAML at all, replace r's list
// wholesale (YAML has no notion of "append to this option's list").
func (y *YAMLHeaderRule) Apply(r *HeaderRule) {
	if y == nil {
		return
	}
	if y.Enabled != nil {
		r.Enabled = *y.Enabled
	}
	if y.OnErrorOnly != nil {
		r.OnErrorOnly = *y.OnErrorOnly
	}
	if y.Exclude != nil {
		r.Exclude = y.Exclude
	}
	if y.Mask != nil {
		r.Mask = y.Mask
	}
}

// YAMLHTTPRules mirrors HTTPRules for a "capture:" YAML subsection -
// embed it in an integration's own yaml.go config struct:
//
//	type yamlConfig struct {
//		Capture capture.YAMLHTTPRules `yaml:"capture"`
//	}
//	...
//	y.Capture.Apply(&rt.rules)
type YAMLHTTPRules struct {
	RequestBody     *YAMLRule       `yaml:"request_body"`
	ResponseBody    *YAMLRule       `yaml:"response_body"`
	RequestHeaders  *YAMLHeaderRule `yaml:"request_headers"`
	ResponseHeaders *YAMLHeaderRule `yaml:"response_headers"`
}

// Apply overlays every YAML-set field of y onto r.
func (y YAMLHTTPRules) Apply(r *HTTPRules) {
	y.RequestBody.Apply(&r.RequestBody)
	y.ResponseBody.Apply(&r.ResponseBody)
	y.RequestHeaders.Apply(&r.RequestHeaders)
	y.ResponseHeaders.Apply(&r.ResponseHeaders)
}
