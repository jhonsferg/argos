package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jhonsferg/argos/samples/inventory-api/inventorypb"
)

// ItemGetter is the read-side persistence interface handlers depend on.
// *ItemStore satisfies it; tests use a fake so the HTTP layer is testable
// without a real Postgres/Redis.
type ItemGetter interface {
	Get(ctx context.Context, id string) (Item, error)
}

// EventPublisherAPI is the subset of *EventPublisher the HTTP layer needs.
type EventPublisherAPI interface {
	PublishStockChanged(ctx context.Context, evt StockChanged) error
}

// API holds the handlers' dependencies: read-side storage, the gRPC client
// used to reserve stock (so one HTTP request produces an HTTP server span,
// a gRPC client+server span pair, and everything ReserveStock itself does),
// and the Kafka publisher.
type API struct {
	items     ItemGetter
	inventory inventorypb.InventoryClient
	events    EventPublisherAPI
}

func NewAPI(items ItemGetter, inventory inventorypb.InventoryClient, events EventPublisherAPI) *API {
	return &API{items: items, inventory: inventory, events: events}
}

func (a *API) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /items/{id}", a.handleGetItem)
	mux.HandleFunc("POST /orders", a.handlePlaceOrder)
	return mux
}

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *API) handleGetItem(w http.ResponseWriter, r *http.Request) {
	item, err := a.items.Get(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, ErrItemNotFound):
		http.Error(w, "item not found", http.StatusNotFound)
		return
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

type placeOrderRequest struct {
	ItemID   string `json:"item_id"`
	Quantity int32  `json:"quantity"`
}

type placeOrderResponse struct {
	ItemID    string `json:"item_id"`
	Remaining int32  `json:"remaining"`
}

// handlePlaceOrder is the rich path: it calls the local gRPC service to
// reserve stock (a real network round trip, same process) - which itself
// runs the reservation inside argos.TraceFunc against Postgres - then, on
// success, publishes a StockChanged event to Kafka. One HTTP request
// produces a trace spanning HTTP -> gRPC -> Postgres -> Kafka producer.
func (a *API) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	var req placeOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := a.inventory.ReserveStock(r.Context(), &inventorypb.ReserveStockRequest{
		ItemId:   req.ItemID,
		Quantity: req.Quantity,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			http.Error(w, "item not found", http.StatusNotFound)
		case codes.FailedPrecondition:
			http.Error(w, "insufficient stock", http.StatusConflict)
		case codes.InvalidArgument:
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			// This is the error path WithCaptureBodyOnError is wired to in
			// main.go - a 500 here also captures the request/response body
			// on the span for debugging.
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	if pubErr := a.events.PublishStockChanged(r.Context(), StockChanged{ItemID: resp.GetItemId(), Remaining: int(resp.GetRemaining())}); pubErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(placeOrderResponse{ItemID: resp.GetItemId(), Remaining: resp.GetRemaining()})
}
