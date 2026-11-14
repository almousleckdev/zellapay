package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Guards are the middleware applied to the authenticated endpoints.
type Guards struct {
	Authenticate httpx.Decorator
	Read         httpx.Decorator // requires the read scope
	Write        httpx.Decorator // requires the webhooks:write scope
	Idempotent   httpx.Decorator
}

// Mount registers the webhook routes on mux.
func (h *Handler) Mount(mux *http.ServeMux, g Guards) {
	mux.Handle("POST /v1/webhook-endpoints",
		httpx.Chain(http.HandlerFunc(h.Create), g.Authenticate, g.Write, g.Idempotent))
	mux.Handle("GET /v1/webhook-endpoints",
		httpx.Chain(http.HandlerFunc(h.List), g.Authenticate, g.Read))
	mux.Handle("POST /v1/webhook-events/{event_id}/replay",
		httpx.Chain(http.HandlerFunc(h.Replay), g.Authenticate, g.Write, g.Idempotent))
}
