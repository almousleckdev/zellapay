package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Guards are the middleware applied to the authenticated endpoints.
type Guards struct {
	Authenticate httpx.Decorator
	Read         httpx.Decorator // requires the read scope
	Write        httpx.Decorator // requires the payouts:write scope
	Idempotent   httpx.Decorator
}

// Mount registers the payout routes on mux.
func (h *Handler) Mount(mux *http.ServeMux, g Guards) {
	mux.Handle("POST /v1/payouts",
		httpx.Chain(http.HandlerFunc(h.Create), g.Authenticate, g.Write, g.Idempotent))
	mux.Handle("POST /v1/payouts/batch",
		httpx.Chain(http.HandlerFunc(h.Batch), g.Authenticate, g.Write, g.Idempotent))
	mux.Handle("GET /v1/payouts",
		httpx.Chain(http.HandlerFunc(h.List), g.Authenticate, g.Read))
	mux.Handle("GET /v1/payouts/{id}",
		httpx.Chain(http.HandlerFunc(h.Get), g.Authenticate, g.Read))
}
