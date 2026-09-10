package argoshttpclient

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	core "github.com/jhonsferg/argos"
)

func TestNewClient_WithYAMLConfig_HeaderCapture(t *testing.T) {
	exp := setTracer(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n" +
		"  httpclient:\n" +
		"    capture:\n" +
		"      request_headers:\n" +
		"        enabled: true\n" +
		"        exclude: [\"Authorization\"]\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := core.FromYAML(path)
	if err != nil {
		t.Fatalf("FromYAML: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.Client(), WithYAMLConfig(cfg))
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-Plain", "visible")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()

	var gotPlain bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		switch string(kv.Key) {
		case "http.request.header.authorization":
			t.Error("Authorization must not be captured (excluded via YAML)")
		case "http.request.header.x-plain":
			gotPlain = true
		}
	}
	if !gotPlain {
		t.Error("expected X-Plain to be captured per the YAML-configured rule")
	}
}
