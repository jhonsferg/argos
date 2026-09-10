// Package argosfiber instruments gofiber/fiber (v2) routers. Fiber runs on
// fasthttp, not net/http, so it can't reuse httpservercore.Middleware or
// Instrumentor.Start(*http.Request) - this calls
// Instrumentor.StartWithCarrier directly with a fasthttp header carrier
// (carrier.go). Fiber resolves routing before invoking handlers (c.Route()
// is already the matched pattern), same as gin/echo - no resolve-after
// trick needed.
package argosfiber

import (
	"net/http"

	"github.com/gofiber/fiber/v2"

	httpservercore "github.com/jhonsferg/argos/integrations/httpserver/core"
	coremiddleware "github.com/jhonsferg/argos/middleware"
)

// Middleware wraps the fiber chain with Argos HTTP server instrumentation,
// resolving the route via c.Route().Path. Register it with
// app.Use(Middleware(...)).
func Middleware(opts ...httpservercore.Option) fiber.Handler {
	instr := httpservercore.New(opts...)

	return func(c *fiber.Ctx) error {
		carrier := requestHeaderCarrier{header: &c.Request().Header}
		ctx, span, start := instr.StartWithCarrier(c.UserContext(), c.Method(), carrier)
		c.SetUserContext(ctx)

		var handlerErr error
		func() {
			defer func() {
				if panicErr := coremiddleware.Handle(ctx, instr.Logger(), recover()); panicErr != nil {
					handlerErr = panicErr
				}
			}()
			handlerErr = c.Next()
		}()

		// Fiber resolves a returned error into a status/body via its
		// ErrorHandler only once this middleware (and every other one)
		// returns, so without forcing it here c.Response().StatusCode()
		// would still read fasthttp's zero-value default instead of the
		// real outcome. Calling it ourselves and still returning handlerErr
		// is safe: fiber's own resolution afterward recomputes the same
		// deterministic (status, body) from (c, handlerErr) and rewrites
		// fasthttp's in-memory response - there's no "already committed"
		// state to double-write at this point in fasthttp's model.
		if handlerErr != nil {
			if err := c.App().Config().ErrorHandler(c, handlerErr); err != nil {
				_ = c.SendStatus(http.StatusInternalServerError)
			}
		}

		route := ""
		if r := c.Route(); r != nil {
			route = r.Path
		}
		instr.End(ctx, span, c.Method(), route, c.Response().StatusCode(), start)
		return handlerErr
	}
}
