// Package argosgin instruments gin routers. gin.Context isn't an
// http.Handler, so unlike the nethttp/chi adapters this one can't reuse
// httpservercore.Instrumentor.Middleware directly - it calls the same
// Start/End building blocks from a gin.HandlerFunc instead. The span/metric
// logic itself still lives entirely in httpservercore.
package argosgin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpservercore "github.com/jhonsferg/argos/integrations/httpserver/core"
	"github.com/jhonsferg/argos/middleware"
)

// Middleware wraps the gin engine with Argos HTTP server instrumentation,
// resolving the route via gin's own c.FullPath(). Register it with
// engine.Use(Middleware(...)).
func Middleware(opts ...httpservercore.Option) gin.HandlerFunc {
	instr := httpservercore.New(opts...)

	return func(c *gin.Context) {
		ctx, span, start := instr.Start(c.Request)
		c.Request = c.Request.WithContext(ctx)

		defer func() {
			if err := middleware.Handle(ctx, instr.Logger(), recover()); err != nil && !c.Writer.Written() {
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			route := c.FullPath()
			instr.End(ctx, span, c.Request.Method, route, c.Writer.Status(), start)
		}()

		c.Next()
	}
}
