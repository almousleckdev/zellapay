package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Service is what the handlers need from the payouts module.
type Service interface {
	Create(ctx context.Context, in payouts.CreateInput) (payouts.Payout, error)
	Get(ctx context.Context, id string) (payouts.Payout, error)
	List(ctx context.Context, q payouts.ListQuery) (payouts.Page, error)
}

// MerchantFunc returns the authenticated merchant of a request.
type MerchantFunc func(*http.Request) (string, bool)

// Handler serves the payout endpoints.
type Handler struct {
	svc      Service
	merchant MerchantFunc
	log      *slog.Logger
}

// New returns a Handler.
func New(svc Service, merchant MerchantFunc, log *slog.Logger) *Handler {
	return &Handler{svc: svc, merchant: merchant, log: log}
}

func (r createRequest) input(merchantID, key string) payouts.CreateInput {
	return payouts.CreateInput{
		MerchantID: merchantID, IdempotencyKey: key, Reference: r.MerchantReference,
		Phone: r.RecipientPhone, Currency: r.Currency, Description: r.Description,
		Amount: r.Amount,
	}
}

// Create handles POST /v1/payouts.
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
	p, err := h.svc.Create(r.Context(), req.input(merchantID, r.Header.Get("Idempotency-Key")))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/payouts/"+p.ID)
	httpx.WriteJSON(w, http.StatusCreated, toResponse(p))
}

// Get handles GET /v1/payouts/{id}. A payout that belongs to another
// merchant is reported as not found.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := h.merchant(r)
	if !ok {
		h.fail(w, r, errNoMerchant)
		return
	}
	p, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err == nil && p.MerchantID != merchantID {
		err = payouts.ErrNotFound
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toResponse(p))
}

// List handles GET /v1/payouts.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := h.merchant(r)
	if !ok {
		h.fail(w, r, errNoMerchant)
		return
	}
	q := payouts.ListQuery{MerchantID: merchantID, After: r.URL.Query().Get("starting_after")}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			httpx.WriteError(w, r, problemBadQuery)
			return
		}
		q.Limit = n
	}
	page, err := h.svc.List(r.Context(), q)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toListResponse(page))
}
