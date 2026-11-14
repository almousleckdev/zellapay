package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Service is what the handlers need from the collections module.
type Service interface {
	Create(ctx context.Context, in collections.CreateInput) (collections.Collection, error)
	Get(ctx context.Context, id string) (collections.Collection, error)
	List(ctx context.Context, q collections.ListQuery) (collections.Page, error)
	HandleCallback(ctx context.Context, operator string, r *http.Request) error
}

// MerchantFunc returns the authenticated merchant of a request.
type MerchantFunc func(*http.Request) (string, bool)

// Handler serves the collections endpoints.
type Handler struct {
	svc      Service
	merchant MerchantFunc
	log      *slog.Logger
}

// New returns a Handler. merchant must come from authentication that ran
// earlier in the chain.
func New(svc Service, merchant MerchantFunc, log *slog.Logger) *Handler {
	return &Handler{svc: svc, merchant: merchant, log: log}
}

// Create handles POST /v1/collections.
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
	c, err := h.svc.Create(r.Context(), collections.CreateInput{
		MerchantID:     merchantID,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Reference:      req.MerchantReference,
		Phone:          req.CustomerPhone,
		Currency:       req.Currency,
		Description:    req.Description,
		Amount:         req.Amount,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/collections/"+c.ID)
	httpx.WriteJSON(w, http.StatusCreated, toResponse(c))
}

// Get handles GET /v1/collections/{id}. A collection that belongs to
// another merchant is reported as not found.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := h.merchant(r)
	if !ok {
		h.fail(w, r, errNoMerchant)
		return
	}
	c, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil || c.MerchantID != merchantID {
		if err == nil {
			err = collections.ErrNotFound
		}
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toResponse(c))
}
