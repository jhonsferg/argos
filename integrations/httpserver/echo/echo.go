// Package argosecho instruments labstack/echo routers. Echo resolves
// routing before invoking its middleware chain, so c.Path() is already the
// matched template here - no resolve-after trick needed, unlike net/http/
// chi/gorilla. But echo.Context isn't *http.Request-shaped, so - like
// argosgin - this calls httpservercore.Instrumentor's Start/End directly
// rather than reusing Middleware(routeFunc).
package argosecho

import (
	"net/http"

	"github.com/labstack/echo/v4"

	httpservercore "github.com/jhonsferg/argos/integrations/httpserver/core"
	coremiddleware "github.com/jhonsferg/argos/middleware"
)

// Middleware wraps the echo chain with Argos HTTP server instrumentation,
// resolving the route via c.Path(). Register it with e.Use(Middleware(...)).
func Middleware(opts ...httpservercore.Option) echo.MiddlewareFunc {
	instr := httpservercore.New(opts...)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx, span, start := instr.Start(c.Request())
			c.SetRequest(c.Request().WithContext(ctx))

			var handlerErr error
			func() {
				defer func() {
					if panicErr := coremiddleware.Handle(ctx, instr.Logger(), recover()); panicErr != nil {
						handlerErr = panicErr
					}
				}()
				handlerErr = next(c)
			}()

			// Echo's own error-to-status mapping runs after every
			// e.Use()-registered middleware returns, so without forcing it
			// here c.Response().Status would still read 0 for a handler
			// that returned an error instead of writing one itself. Calling
			// c.Error ourselves is idempotent - Echo's default error
			// handler checks Response().Committed before writing, so the
			// outer machinery doesn't double-write once we've done this.
			if handlerErr != nil && !c.Response().Committed {
				c.Error(handlerErr)
			}

			status := c.Response().Status
			if status == 0 {
				status = http.StatusOK
			}
			instr.End(ctx, span, c.Request().Method, c.Path(), status, start)
			return handlerErr
		}
	}
}
