package keysapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Mount registers the key routes. session guards each one.
func (h *Handler) Mount(mux *http.ServeMux, session httpx.Decorator) {
	route := func(pattern string, f http.HandlerFunc) {
		mux.Handle(pattern, httpx.Chain(f, noStore, session))
	}
	route("GET /dashboard/api-keys", h.List)
	route("POST /dashboard/api-keys", h.Create)
	route("POST /dashboard/api-keys/{id}/rotate", h.Rotate)
	route("DELETE /dashboard/api-keys/{id}", h.Revoke)
}

// noStore stops anything from caching a response that may hold a secret.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
