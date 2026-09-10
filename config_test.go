package argos

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithYAMLConfig_LayersOntoChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "service_name: from-yaml\nsample_ratio: 0.5\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := defaultConfig()
	WithServiceVersion("1.2.3")(&cfg)
	WithYAMLConfig(path)(&cfg)
	WithSampleRatio(0.9)(&cfg)

	if cfg.err != nil {
		t.Fatalf("unexpected cfg.err: %v", cfg.err)
	}
	if cfg.ServiceName != "from-yaml" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "from-yaml")
	}
	if cfg.ServiceVersion != "1.2.3" {
		t.Errorf("ServiceVersion = %q, want %q (set before WithYAMLConfig, must survive)", cfg.ServiceVersion, "1.2.3")
	}
	if cfg.SampleRatio != 0.9 {
		t.Errorf("SampleRatio = %v, want 0.9 (set after WithYAMLConfig, must override)", cfg.SampleRatio)
	}
}

func TestWithYAMLConfig_MissingFileSetsErr(t *testing.T) {
	cfg := defaultConfig()
	WithYAMLConfig(filepath.Join(t.TempDir(), "does-not-exist.yaml"))(&cfg)

	if cfg.err == nil {
		t.Fatal("expected cfg.err to be set for a missing file")
	}
}

func TestDecodeIntegrationConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n  sql:\n    query_text: true\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := defaultConfig()
	WithYAMLConfig(path)(&cfg)
	if cfg.err != nil {
		t.Fatalf("unexpected cfg.err: %v", cfg.err)
	}

	var got struct {
		QueryText *bool `yaml:"query_text"`
	}
	ok, err := DecodeIntegrationConfig(cfg, "sql", &got)
	if err != nil {
		t.Fatalf("DecodeIntegrationConfig: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for a present section")
	}
	if got.QueryText == nil || !*got.QueryText {
		t.Errorf("QueryText = %v, want true", got.QueryText)
	}

	var absent struct{}
	ok, err = DecodeIntegrationConfig(cfg, "redis", &absent)
	if err != nil {
		t.Fatalf("DecodeIntegrationConfig(absent): %v", err)
	}
	if ok {
		t.Error("expected ok=false for an absent section")
	}
}
