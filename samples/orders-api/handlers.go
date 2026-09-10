package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	argos "github.com/jhonsferg/argos"
)

// Store is the persistence interface handlers depend on. *OrderStore
// satisfies it; tests use a fake so the HTTP layer is testable without a
// real Postgres/Redis.
type Store interface {
	Create(ctx context.Context, o Order) (Order, error)
	Get(ctx context.Context, id int64) (Order, error)
}

// API holds the handlers' dependencies: the store and the correlated logger
// argos.Init produced.
type API struct {
	store  Store
	logger argos.Logger
}

func NewAPI(store Store, logger argos.Logger) *API {
	return &API{store: store, logger: logger}
}

// Routes builds the enhanced net/http.ServeMux argosnethttp.Middleware wraps.
func (a *API) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("POST /orders", a.handleCreateOrder)
	mux.HandleFunc("GET /orders/{id}", a.handleGetOrder)
	return mux
}

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *API) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var o Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	created, err := a.store.Create(r.Context(), o)
	if err != nil {
		a.logger.Error(r.Context(), "create order failed", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (a *API) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	order, err := a.store.Get(r.Context(), id)
	switch {
	case errors.Is(err, ErrOrderNotFound):
		http.Error(w, "order not found", http.StatusNotFound)
		return
	case err != nil:
		a.logger.Error(r.Context(), "get order failed", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(order)
}
