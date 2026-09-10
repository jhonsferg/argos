package main

import (
	"context"
	"errors"
	"path"
	"time"

	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"

	argos "github.com/jhonsferg/argos"
	argosfiber "github.com/jhonsferg/argos/integrations/httpserver/fiber"
)

// Puller and Store are the interfaces handlers depend on. *ReportPuller and
// *ReportStore satisfy them; tests use fakes so the HTTP layer is testable
// without a real SFTP server/Cassandra.
type Puller interface {
	Pull(ctx context.Context, path string) (int64, error)
}

type Store interface {
	Save(ctx context.Context, r ReportRecord) error
	Get(ctx context.Context, filename string) (ReportRecord, error)
}

type API struct {
	puller Puller
	store  Store
	logger argos.Logger
}

func NewAPI(puller Puller, store Store, logger argos.Logger) *API {
	return &API{puller: puller, store: store, logger: logger}
}

type pullRequest struct {
	Path string `json:"path"`
}

func (a *API) Routes() *fiber.App {
	app := fiber.New()
	app.Use(argosfiber.Middleware())
	app.Get("/healthz", a.handleHealth)
	app.Post("/reports/pull", a.handlePullReport)
	app.Get("/reports/:filename", a.handleGetReport)
	return app
}

func (a *API) handleHealth(c *fiber.Ctx) error {
	return c.SendString("ok")
}

func (a *API) handlePullReport(c *fiber.Ctx) error {
	var req pullRequest
	if err := c.BodyParser(&req); err != nil || req.Path == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path is required"})
	}

	// argosfiber.Middleware sets the span-carrying context via
	// c.SetUserContext - that's what downstream handlers must read to stay
	// inside the request span, not the raw fasthttp c.Context().
	ctx := c.UserContext()

	size, err := a.puller.Pull(ctx, req.Path)
	if err != nil {
		a.logger.Error(ctx, "pull report failed", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "pull failed"})
	}

	record := ReportRecord{Filename: path.Base(req.Path), Size: size, PulledAt: time.Now().UTC()}
	if err := a.store.Save(ctx, record); err != nil {
		a.logger.Error(ctx, "save report record failed", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}

	return c.JSON(record)
}

func (a *API) handleGetReport(c *fiber.Ctx) error {
	ctx := c.UserContext()
	filename := c.Params("filename")

	record, err := a.store.Get(ctx, filename)
	switch {
	case errors.Is(err, gocql.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "report not found"})
	case err != nil:
		a.logger.Error(ctx, "get report failed", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}

	return c.JSON(record)
}
