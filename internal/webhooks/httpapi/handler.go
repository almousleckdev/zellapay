// Package httpapi exposes webhook endpoint management over HTTP according to
// api/openapi.yaml.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/webhooks"
)

// Service is what the handlers need from the webhooks module.
type Service interface {
	CreateEndpoint(
		ctx context.Context, in webhooks.CreateEndpointInput,
	) (webhooks.CreatedEndpoint, error)
	ListEndpoints(ctx context.Context, merchantID string) ([]webhooks.Endpoint, error)
	Replay(ctx context.Context, merchantID, eventID string) (int, error)
}

// MerchantFunc returns the authenticated merchant of a request.
type MerchantFunc func(*http.Request) (string, bool)

// Handler serves the webhook endpoints.
type Handler struct {
	svc      Service
	merchant MerchantFunc
	log      *slog.Logger
}

// New returns a Handler.
func New(svc Service, merchant MerchantFunc, log *slog.Logger) *Handler {
	return &Handler{svc: svc, merchant: merchant, log: log}
}

type createRequest struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Description string   `json:"description"`
}

type endpointResponse struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Events      []string  `json:"events"`
	Description string    `json:"description"`
	Secret      string    `json:"secret,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func toResponse(e webhooks.Endpoint, secret string) endpointResponse {
	return endpointResponse{
		ID: e.ID, URL: e.URL, Events: e.Events, Description: e.Description,
		Secret: secret, CreatedAt: e.CreatedAt.UTC(),
	}
}

// Create handles POST /v1/webhook-endpoints. The signing secret appears in
// this response only.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := h.merchant(r)
	if !ok {
		h.fail(w, r, errNoMerchant)
		return
	}
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, problemBadJSON)
		return
	}
	ep, err := h.svc.CreateEndpoint(r.Context(), webhooks.CreateEndpointInput{
		MerchantID: merchantID, URL: req.URL, Events: req.Events, Description: req.Description,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toResponse(ep.Endpoint, ep.Secret))
}

// List handles GET /v1/webhook-endpoints. Secrets are never returned.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := h.merchant(r)
	if !ok {
		h.fail(w, r, errNoMerchant)
		return
	}
	eps, err := h.svc.ListEndpoints(r.Context(), merchantID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]endpointResponse, len(eps))
	for i, e := range eps {
		out[i] = toResponse(e, "")
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

// Replay handles POST /v1/webhook-events/{event_id}/replay.
func (h *Handler) Replay(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := h.merchant(r)
	if !ok {
		h.fail(w, r, errNoMerchant)
		return
	}
	if _, err := h.svc.Replay(r.Context(), merchantID, r.PathValue("event_id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
