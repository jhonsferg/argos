package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gocql/gocql"

	argos "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/logging"
)

func newHTTPRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

type fakePuller struct {
	size int64
	err  error
}

func (f *fakePuller) Pull(_ context.Context, _ string) (int64, error) {
	return f.size, f.err
}

type fakeStore struct {
	records map[string]ReportRecord
}

func newFakeStore() *fakeStore {
	return &fakeStore{records: make(map[string]ReportRecord)}
}

func (f *fakeStore) Save(_ context.Context, r ReportRecord) error {
	f.records[r.Filename] = r
	return nil
}

func (f *fakeStore) Get(_ context.Context, filename string) (ReportRecord, error) {
	r, ok := f.records[filename]
	if !ok {
		return ReportRecord{}, gocql.ErrNotFound
	}
	return r, nil
}

func testLogger() argos.Logger {
	return logging.NewZerolog(bytes.NewBuffer(nil), 0, nil)
}

func TestHandlePullReport(t *testing.T) {
	puller := &fakePuller{size: 1234}
	store := newFakeStore()
	api := NewAPI(puller, store, testLogger())
	app := api.Routes()

	body := `{"path":"/upload/sample-report.csv"}`
	req := newHTTPRequest("POST", "/reports/pull", body)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data, _ := io.ReadAll(resp.Body)
	var got ReportRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Filename != "sample-report.csv" || got.Size != 1234 {
		t.Errorf("unexpected record: %+v", got)
	}
}

func TestHandlePullReport_PullFailure(t *testing.T) {
	puller := &fakePuller{err: errors.New("connection refused")}
	api := NewAPI(puller, newFakeStore(), testLogger())
	app := api.Routes()

	req := newHTTPRequest("POST", "/reports/pull", `{"path":"/upload/sample-report.csv"}`)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 502 {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
}

func TestHandleGetReport_NotFound(t *testing.T) {
	api := NewAPI(&fakePuller{}, newFakeStore(), testLogger())
	app := api.Routes()

	req := newHTTPRequest("GET", "/reports/missing.csv", "")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHandleHealth(t *testing.T) {
	api := NewAPI(&fakePuller{}, newFakeStore(), testLogger())
	app := api.Routes()

	req := newHTTPRequest("GET", "/healthz", "")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
