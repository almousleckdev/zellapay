package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Guards are the middleware applied to the authenticated endpoints. They
// are supplied by the composition root so this package stays independent of
// how authentication and idempotency are implemented.
type Guards struct {
	Authenticate httpx.Decorator
	Read         httpx.Decorator // requires the read scope
	Write        httpx.Decorator // requires the collections:write scope
	Idempotent   httpx.Decorator
}

// Mount registers the collections routes on mux.
func (h *Handler) Mount(mux *http.ServeMux, g Guards) {
	mux.Handle("POST /v1/collections",
		httpx.Chain(http.HandlerFunc(h.Create), g.Authenticate, g.Write, g.Idempotent))
	mux.Handle("GET /v1/collections",
		httpx.Chain(http.HandlerFunc(h.List), g.Authenticate, g.Read))
	mux.Handle("GET /v1/collections/{id}",
		httpx.Chain(http.HandlerFunc(h.Get), g.Authenticate, g.Read))
	mux.HandleFunc("POST /callbacks/{operator}", h.Callback)
}
