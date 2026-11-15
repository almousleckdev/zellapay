package httpapi

import (
	"context"
	"net/http"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity/identityhttp"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

type statusAction func(context.Context, merchants.StatusChange) (merchants.Merchant, error)

// Suspend stops an active merchant. A typed reason is required.
func (h *Handler) Suspend(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, h.d.Merchants.Suspend)
}

// Restore returns a suspended merchant to active. A reason is required.
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, h.d.Merchants.Restore)
}

func (h *Handler) changeStatus(w http.ResponseWriter, r *http.Request, do statusAction) {
	var req reasonRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, identityhttp.ProblemBadJSON)
		return
	}
	m, err := do(r.Context(), merchants.StatusChange{
		MerchantID: r.PathValue("id"),
		By:         audit.Actor{Kind: audit.ActorStaff, ID: callerFrom(r.Context()).User.ID},
		Reason:     req.Reason,
		Source:     h.source(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, merchantView(m))
}
