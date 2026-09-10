package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"

	argos "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/logging"
)

type fakeStore struct {
	shipments map[string]Shipment
}

func newFakeStore() *fakeStore {
	return &fakeStore{shipments: make(map[string]Shipment)}
}

func (f *fakeStore) Upsert(_ context.Context, sh Shipment) (Shipment, error) {
	f.shipments[sh.OrderID] = sh
	return sh, nil
}

func (f *fakeStore) Get(_ context.Context, orderID string) (Shipment, error) {
	sh, ok := f.shipments[orderID]
	if !ok {
		return Shipment{}, mongo.ErrNoDocuments
	}
	return sh, nil
}

type fakePublisher struct {
	published []Shipment
}

func (f *fakePublisher) PublishShipmentUpdated(_ context.Context, sh Shipment) error {
	f.published = append(f.published, sh)
	return nil
}

func testLogger() argos.Logger {
	return logging.NewZerolog(bytes.NewBuffer(nil), 0, nil)
}

func TestHandleUpsertShipment(t *testing.T) {
	store := newFakeStore()
	pub := &fakePublisher{}
	api := NewAPI(store, pub, testLogger())

	body := `{"order_id":"order-1","status":"in_transit"}`
	req := httptest.NewRequest(http.MethodPost, "/shipments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got Shipment
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.OrderID != "order-1" || got.Status != "in_transit" {
		t.Errorf("unexpected shipment: %+v", got)
	}
	if len(pub.published) != 1 {
		t.Errorf("expected 1 published event, got %d", len(pub.published))
	}
}

func TestHandleUpsertShipment_MissingOrderID(t *testing.T) {
	api := NewAPI(newFakeStore(), &fakePublisher{}, testLogger())

	body := `{"status":"in_transit"}`
	req := httptest.NewRequest(http.MethodPost, "/shipments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetShipment_Found(t *testing.T) {
	store := newFakeStore()
	store.shipments["order-1"] = Shipment{OrderID: "order-1", Status: "delivered"}
	api := NewAPI(store, &fakePublisher{}, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/shipments/order-1", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleGetShipment_NotFound(t *testing.T) {
	api := NewAPI(newFakeStore(), &fakePublisher{}, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/shipments/does-not-exist", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleHealth(t *testing.T) {
	api := NewAPI(newFakeStore(), &fakePublisher{}, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
