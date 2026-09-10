package argosfiber

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// BenchmarkHandler_Baseline measures a plain fiber app with no Argos
// middleware registered - the "without Argos" comparison point.
func BenchmarkHandler_Baseline(b *testing.B) {
	app := fiber.New()
	app.Get("/items/:id", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := app.Test(req, -1); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHandler_Instrumented measures the same app with Middleware
// registered, documenting the allocation cost it adds per request.
func BenchmarkHandler_Instrumented(b *testing.B) {
	app := fiber.New()
	app.Use(Middleware())
	app.Get("/items/:id", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := app.Test(req, -1); err != nil {
			b.Fatal(err)
		}
	}
}
