package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// chiCtx returns the *chi.Context attached to the request, if any.
// Wrapped here so the middleware module doesn't import chi twice.
func chiCtx(r *http.Request) *chi.Context {
	return chi.RouteContext(r.Context())
}
