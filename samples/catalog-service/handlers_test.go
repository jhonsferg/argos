package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	argos "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/logging"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeStore and fakePublisher let the HTTP layer be tested without a real
// Postgres/Kafka.
type fakeStore struct {
	items  map[uint]Item
	nextID uint
}

func newFakeStore() *fakeStore {
	return &fakeStore{items: make(map[uint]Item)}
}

func (f *fakeStore) Create(_ context.Context, item Item) (Item, error) {
	f.nextID++
	item.ID = f.nextID
	f.items[item.ID] = item
	return item, nil
}

func (f *fakeStore) Get(_ context.Context, id uint) (Item, error) {
	item, ok := f.items[id]
	if !ok {
		return Item{}, gorm.ErrRecordNotFound
	}
	return item, nil
}

type fakePublisher struct {
	published []Item
	err       error
}

func (f *fakePublisher) PublishItemCreated(_ context.Context, item Item) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, item)
	return nil
}

func testLogger() argos.Logger {
	return logging.NewZerolog(bytes.NewBuffer(nil), 0, nil)
}

func TestHandleCreateItem(t *testing.T) {
	store := newFakeStore()
	pub := &fakePublisher{}
	api := NewAPI(store, pub, testLogger())

	body := `{"name":"widget","price_cents":1999}`
	req := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got Item
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID == 0 || got.Name != "widget" || got.Price != 1999 {
		t.Errorf("unexpected item: %+v", got)
	}
	if len(pub.published) != 1 {
		t.Errorf("expected 1 published event, got %d", len(pub.published))
	}
}

func TestHandleCreateItem_PublishFailureStillCreates(t *testing.T) {
	store := newFakeStore()
	pub := &fakePublisher{err: errors.New("broker unreachable")}
	api := NewAPI(store, pub, testLogger())

	body := `{"name":"widget","price_cents":1999}`
	req := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d - a publish failure must not roll back the create", rec.Code, http.StatusCreated)
	}
}

func TestHandleGetItem_Found(t *testing.T) {
	store := newFakeStore()
	store.items[7] = Item{ID: 7, Name: "gizmo", Price: 500}
	api := NewAPI(store, &fakePublisher{}, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleGetItem_NotFound(t *testing.T) {
	api := NewAPI(newFakeStore(), &fakePublisher{}, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/items/999", nil)
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
