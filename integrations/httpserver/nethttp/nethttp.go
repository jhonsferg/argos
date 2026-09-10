// Package argosnethttp instruments stdlib net/http handlers. It's a thin
// adapter over httpservercore - see that package for the actual span/metric
// logic, which every router adapter shares.
package argosnethttp

import (
	"net/http"
	"strings"

	httpservercore "github.com/jhonsferg/argos/integrations/httpserver/core"
)

// Middleware wraps next with Argos HTTP server instrumentation. It resolves
// the route from r.Pattern, populated by the Go 1.22+ enhanced ServeMux once
// it has dispatched the request - see httpservercore's package doc for why
// this must be read after, not before, the handler runs. When Pattern is
// empty (a plain mux, or no pattern registered), no route is reported: per
// OTel semantic conventions, http.route must not be set to the raw URL path,
// since that reintroduces the high-cardinality problem the attribute exists
// to avoid. The span still gets a method-only name in that case.
func Middleware(opts ...httpservercore.Option) func(http.Handler) http.Handler {
	return httpservercore.New(opts...).Middleware(routeFunc)
}

// routeFunc strips the optional "METHOD " prefix ServeMux patterns can carry
// (e.g. "GET /items/{id}") so http.route holds just the path template - the
// method is already a separate attribute and part of the span name on its
// own.
func routeFunc(r *http.Request) string {
	pattern := r.Pattern
	if i := strings.IndexByte(pattern, ' '); i != -1 {
		return pattern[i+1:]
	}
	return pattern
}
