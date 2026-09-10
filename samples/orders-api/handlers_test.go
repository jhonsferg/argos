package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	argos "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/logging"
)

// fakeStore is an in-memory Store used to test the HTTP handler layer
// without a real Postgres/Redis.
type fakeStore struct {
	orders map[int64]Order
	nextID int64
	getErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{orders: make(map[int64]Order)}
}

func (f *fakeStore) Create(_ context.Context, o Order) (Order, error) {
	f.nextID++
	o.ID = f.nextID
	f.orders[o.ID] = o
	return o, nil
}

func (f *fakeStore) Get(_ context.Context, id int64) (Order, error) {
	if f.getErr != nil {
		return Order{}, f.getErr
	}
	o, ok := f.orders[id]
	if !ok {
		return Order{}, ErrOrderNotFound
	}
	return o, nil
}

func testLogger() argos.Logger {
	return logging.NewZerolog(bytes.NewBuffer(nil), 0, nil)
}

func TestHandleCreateOrder(t *testing.T) {
	store := newFakeStore()
	api := NewAPI(store, testLogger())

	body := `{"customer_name":"ana","item":"widget","quantity":3}`
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got Order
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID == 0 || got.CustomerName != "ana" || got.Quantity != 3 {
		t.Errorf("unexpected order: %+v", got)
	}
}

func TestHandleGetOrder_Found(t *testing.T) {
	store := newFakeStore()
	store.orders[42] = Order{ID: 42, CustomerName: "ana", Item: "widget", Quantity: 1}
	api := NewAPI(store, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/orders/42", nil)
	rec := httptest.NewRecorder()

	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got Order
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != 42 {
		t.Errorf("id = %d, want 42", got.ID)
	}
}

func TestHandleGetOrder_NotFound(t *testing.T) {
	api := NewAPI(newFakeStore(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/orders/999", nil)
	rec := httptest.NewRecorder()

	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleGetOrder_InvalidID(t *testing.T) {
	api := NewAPI(newFakeStore(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/orders/not-a-number", nil)
	rec := httptest.NewRecorder()

	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleHealth(t *testing.T) {
	api := NewAPI(newFakeStore(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
