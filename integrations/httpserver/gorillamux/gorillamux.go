// Package argosgorillamux instruments gorilla/mux routers. Like argoschi, it
// is a thin adapter over httpservercore - see that package for the actual
// span/metric logic, which every router adapter shares.
package argosgorillamux

import (
	"net/http"

	"github.com/gorilla/mux"

	httpservercore "github.com/jhonsferg/argos/integrations/httpserver/core"
)

// Middleware wraps next with Argos HTTP server instrumentation, resolving
// the route via mux.CurrentRoute. Register it with r.Use(...); like any
// gorilla/mux middleware, it must be mounted before the routes it should
// cover.
func Middleware(opts ...httpservercore.Option) func(http.Handler) http.Handler {
	return httpservercore.New(opts...).Middleware(routeFunc)
}

func routeFunc(r *http.Request) string {
	route := mux.CurrentRoute(r)
	if route == nil {
		return ""
	}
	tpl, err := route.GetPathTemplate()
	if err != nil {
		return ""
	}
	return tpl
}
