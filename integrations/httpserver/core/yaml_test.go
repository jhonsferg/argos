package httpservercore

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	core "github.com/jhonsferg/argos"
)

func TestNew_WithYAMLConfig_HeaderCapture(t *testing.T) {
	exp := setTracer(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "integrations:\n" +
		"  httpserver:\n" +
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

	instr := New(WithYAMLConfig(cfg))
	handler := instr.Middleware(staticRoute("/ok"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-Plain", "visible")
	handler.ServeHTTP(httptest.NewRecorder(), req)

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
