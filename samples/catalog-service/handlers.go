package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	argos "github.com/jhonsferg/argos"
	argosgin "github.com/jhonsferg/argos/integrations/httpserver/gin"
)

// Store and Publisher are the interfaces handlers depend on. *ItemStore and
// *EventPublisher satisfy them; tests use fakes so the HTTP layer is
// testable without a real Postgres/Kafka.
type Store interface {
	Create(ctx context.Context, item Item) (Item, error)
	Get(ctx context.Context, id uint) (Item, error)
}

type Publisher interface {
	PublishItemCreated(ctx context.Context, item Item) error
}

type API struct {
	store     Store
	publisher Publisher
	logger    argos.Logger
}

func NewAPI(store Store, publisher Publisher, logger argos.Logger) *API {
	return &API{store: store, publisher: publisher, logger: logger}
}

// Routes builds the gin.Engine argosgin.Middleware is registered on.
func (a *API) Routes() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), argosgin.Middleware())
	r.GET("/healthz", a.handleHealth)
	r.POST("/items", a.handleCreateItem)
	r.GET("/items/:id", a.handleGetItem)
	return r
}

func (a *API) handleHealth(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

func (a *API) handleCreateItem(c *gin.Context) {
	var item Item
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	created, err := a.store.Create(c.Request.Context(), item)
	if err != nil {
		a.logger.Error(c.Request.Context(), "create item failed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// A publish failure doesn't roll back the create: the row is the source
	// of truth, the event is a best-effort side effect. Fail-open, log it.
	if err := a.publisher.PublishItemCreated(c.Request.Context(), created); err != nil {
		a.logger.Error(c.Request.Context(), "publish item created event failed", err)
	}

	c.JSON(http.StatusCreated, created)
}

func (a *API) handleGetItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	item, err := a.store.Get(c.Request.Context(), uint(id))
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	case err != nil:
		a.logger.Error(c.Request.Context(), "get item failed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, item)
}
