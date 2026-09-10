package argoshttpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jhonsferg/argos/capture"
)

func TestRoundTrip_CaptureBodyOnError_CapturesOnFailure(t *testing.T) {
	exp := setTracer(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"db down"}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.Client(), WithCaptureBodyOnError(1024))
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"user_id":42}`))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(respBody) != `{"error":"db down"}` {
		t.Fatalf("response body reaching the caller = %q, want the full original body despite capture", respBody)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	var gotReqBody, gotRespBody bool
	for _, kv := range spans[0].Attributes {
		switch string(kv.Key) {
		case "argos.http.request.body":
			if kv.Value.AsString() == `{"user_id":42}` {
				gotReqBody = true
			}
		case "argos.http.response.body":
			if kv.Value.AsString() == `{"error":"db down"}` {
				gotRespBody = true
			}
		}
	}
	if !gotReqBody {
		t.Error("expected argos.http.request.body attribute on a 500 response")
	}
	if !gotRespBody {
		t.Error("expected argos.http.response.body attribute on a 500 response")
	}
}

func TestRoundTrip_CaptureBodyOnError_NoCaptureOnSuccess(t *testing.T) {
	exp := setTracer(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.Client(), WithCaptureBodyOnError(1024))
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"user_id":42}`))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(respBody) != `{"ok":true}` {
		t.Fatalf("response body reaching the caller = %q, want unaffected", respBody)
	}

	spans := exp.GetSpans()
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "argos.http.request.body" || string(kv.Key) == "argos.http.response.body" {
			t.Errorf("unexpected captured-body attribute %q on a 200 response", kv.Key)
		}
	}
}

func TestRoundTrip_WithCapture_HeadersExcludeAndMask(t *testing.T) {
	exp := setTracer(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Response-Plain", "visible")
		w.Header().Set("Set-Cookie", "session=secret")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.Client(), WithCapture(capture.HTTPRules{
		RequestHeaders: capture.HeaderRule{
			Enabled: true,
			Exclude: []string{"Authorization"},
			Mask:    []string{"X-Api-Key"},
		},
		ResponseHeaders: capture.HeaderRule{
			Enabled: true,
			Exclude: []string{"Set-Cookie"},
		},
	}))
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-Api-Key", "key-123")
	req.Header.Set("X-Plain", "visible")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()

	attrs := map[string]string{}
	for _, kv := range exp.GetSpans()[0].Attributes {
		if vals := kv.Value.AsStringSlice(); len(vals) > 0 {
			attrs[string(kv.Key)] = vals[0]
		}
	}
	if _, ok := attrs["http.request.header.authorization"]; ok {
		t.Error("Authorization must never be captured (Exclude)")
	}
	if got := attrs["http.request.header.x-api-key"]; got != "***" {
		t.Errorf("X-Api-Key = %q, want masked", got)
	}
	if got := attrs["http.request.header.x-plain"]; got != "visible" {
		t.Errorf("X-Plain = %q, want unmasked", got)
	}
	if _, ok := attrs["http.response.header.set-cookie"]; ok {
		t.Error("Set-Cookie must never be captured (Exclude)")
	}
	if got := attrs["http.response.header.x-response-plain"]; got != "visible" {
		t.Errorf("X-Response-Plain = %q, want unmasked", got)
	}
}

func TestRoundTrip_WithCapture_HeadersOnErrorOnly(t *testing.T) {
	exp := setTracer(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.Client(), WithCapture(capture.HTTPRules{
		RequestHeaders: capture.HeaderRule{Enabled: true, OnErrorOnly: true},
	}))
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("X-Plain", "visible")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()

	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "http.request.header.x-plain" {
			t.Error("headers must not be captured on success when OnErrorOnly is set")
		}
	}
}

func TestRoundTrip_NoCaptureConfigured_NoOverhead(t *testing.T) {
	exp := setTracer(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.Client()) // no WithCaptureBodyOnError
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`sensitive`))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()

	spans := exp.GetSpans()
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == "argos.http.request.body" || string(kv.Key) == "argos.http.response.body" {
			t.Error("capture attributes must not appear when WithCaptureBodyOnError was never configured")
		}
	}
}
