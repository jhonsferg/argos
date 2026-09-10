package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/v2/mongo"

	argos "github.com/jhonsferg/argos"
	argosecho "github.com/jhonsferg/argos/integrations/httpserver/echo"
)

// Store and Publisher are the interfaces handlers depend on. *ShipmentStore
// and *EventPublisher satisfy them; tests use fakes so the HTTP layer is
// testable without a real MongoDB/Pub-Sub.
type Store interface {
	Upsert(ctx context.Context, sh Shipment) (Shipment, error)
	Get(ctx context.Context, orderID string) (Shipment, error)
}

type Publisher interface {
	PublishShipmentUpdated(ctx context.Context, sh Shipment) error
}

type API struct {
	store     Store
	publisher Publisher
	logger    argos.Logger
}

func NewAPI(store Store, publisher Publisher, logger argos.Logger) *API {
	return &API{store: store, publisher: publisher, logger: logger}
}

func (a *API) Routes() *echo.Echo {
	e := echo.New()
	e.Use(argosecho.Middleware())
	e.GET("/healthz", a.handleHealth)
	e.POST("/shipments", a.handleUpsertShipment)
	e.GET("/shipments/:order_id", a.handleGetShipment)
	return e
}

func (a *API) handleHealth(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func (a *API) handleUpsertShipment(c echo.Context) error {
	var sh Shipment
	if err := c.Bind(&sh); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	if sh.OrderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "order_id is required"})
	}

	saved, err := a.store.Upsert(c.Request().Context(), sh)
	if err != nil {
		a.logger.Error(c.Request().Context(), "upsert shipment failed", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
	}

	// A publish failure doesn't roll back the write: the document is the
	// source of truth, the event is a best-effort side effect.
	if err := a.publisher.PublishShipmentUpdated(c.Request().Context(), saved); err != nil {
		a.logger.Error(c.Request().Context(), "publish shipment updated event failed", err)
	}

	return c.JSON(http.StatusOK, saved)
}

func (a *API) handleGetShipment(c echo.Context) error {
	orderID := c.Param("order_id")

	sh, err := a.store.Get(c.Request().Context(), orderID)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return c.JSON(http.StatusNotFound, echo.Map{"error": "shipment not found"})
	case err != nil:
		a.logger.Error(c.Request().Context(), "get shipment failed", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
	}

	return c.JSON(http.StatusOK, sh)
}
