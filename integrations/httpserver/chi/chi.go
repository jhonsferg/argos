// Package argoschi instruments go-chi/chi routers. It's a thin adapter over
// httpservercore - see that package for the actual span/metric logic, which
// every router adapter shares.
package argoschi

import (
	"net/http"

	chirouter "github.com/go-chi/chi/v5"

	httpservercore "github.com/jhonsferg/argos/integrations/httpserver/core"
)

// Middleware wraps next with Argos HTTP server instrumentation, resolving
// the route via chi's RouteContext. Register it with r.Use(...); like any
// chi middleware, it must be mounted before the routes it should cover.
func Middleware(opts ...httpservercore.Option) func(http.Handler) http.Handler {
	return httpservercore.New(opts...).Middleware(routeFunc)
}

func routeFunc(r *http.Request) string {
	rctx := chirouter.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}
	return rctx.RoutePattern()
}
