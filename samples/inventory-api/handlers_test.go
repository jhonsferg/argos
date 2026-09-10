package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jhonsferg/argos/samples/inventory-api/inventorypb"
)

// fakeItemGetter implements ItemGetter for testing the HTTP layer without a
// real Postgres/Redis.
type fakeItemGetter struct {
	items map[string]Item
}

func (f *fakeItemGetter) Get(_ context.Context, id string) (Item, error) {
	it, ok := f.items[id]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	return it, nil
}

// fakeInventoryClient implements inventorypb.InventoryClient for testing
// the HTTP layer without a real gRPC backend.
type fakeInventoryClient struct {
	reserveErr error
	remaining  int32
}

func (f *fakeInventoryClient) ReserveStock(_ context.Context, in *inventorypb.ReserveStockRequest, _ ...grpc.CallOption) (*inventorypb.ReserveStockResponse, error) {
	if f.reserveErr != nil {
		return nil, f.reserveErr
	}
	return &inventorypb.ReserveStockResponse{ItemId: in.GetItemId(), Remaining: f.remaining}, nil
}

// fakeEventPublisher implements EventPublisherAPI, recording what it was
// asked to publish.
type fakeEventPublisher struct {
	published  []StockChanged
	publishErr error
}

func (f *fakeEventPublisher) PublishStockChanged(_ context.Context, evt StockChanged) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	f.published = append(f.published, evt)
	return nil
}

func TestHandleGetItem_Found(t *testing.T) {
	items := &fakeItemGetter{items: map[string]Item{"widget": {ID: "widget", Name: "Widget", Stock: 10}}}
	api := NewAPI(items, &fakeInventoryClient{}, &fakeEventPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/items/widget", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got Item
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Stock != 10 {
		t.Errorf("unexpected item: %+v", got)
	}
}

func TestHandleGetItem_NotFound(t *testing.T) {
	api := NewAPI(&fakeItemGetter{items: map[string]Item{}}, &fakeInventoryClient{}, &fakeEventPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/items/missing", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlePlaceOrder_Success(t *testing.T) {
	events := &fakeEventPublisher{}
	api := NewAPI(&fakeItemGetter{}, &fakeInventoryClient{remaining: 99}, events)

	body := `{"item_id":"widget","quantity":1}`
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got placeOrderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Remaining != 99 {
		t.Errorf("unexpected response: %+v", got)
	}
	if len(events.published) != 1 || events.published[0].Remaining != 99 {
		t.Errorf("expected one StockChanged event published, got %+v", events.published)
	}
}

func TestHandlePlaceOrder_InsufficientStock(t *testing.T) {
	client := &fakeInventoryClient{reserveErr: status.Error(codes.FailedPrecondition, "insufficient stock")}
	api := NewAPI(&fakeItemGetter{}, client, &fakeEventPublisher{})

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"item_id":"widget","quantity":1000}`))
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandlePlaceOrder_ItemNotFound(t *testing.T) {
	client := &fakeInventoryClient{reserveErr: status.Error(codes.NotFound, "item not found")}
	api := NewAPI(&fakeItemGetter{}, client, &fakeEventPublisher{})

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"item_id":"missing","quantity":1}`))
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlePlaceOrder_InvalidBody(t *testing.T) {
	api := NewAPI(&fakeItemGetter{}, &fakeInventoryClient{}, &fakeEventPublisher{})

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`not json`))
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleHealth(t *testing.T) {
	api := NewAPI(&fakeItemGetter{}, &fakeInventoryClient{}, &fakeEventPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
