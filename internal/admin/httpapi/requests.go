package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/identity/identityhttp"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

func (h *Handler) origin(r *http.Request, reason string) merchants.Origin {
	return merchants.Origin{By: h.actor(r), Reason: reason, Source: h.source(r)}
}

func (h *Handler) view(r *http.Request, c merchants.ChangeRequest) requestJSON {
	return requestJSON{
		ID: c.ID, MerchantID: c.MerchantID, MerchantName: c.MerchantName, Kind: string(c.Kind),
		FeeBPS: c.FeeBPS, PayoutFeeBPS: c.PayoutFeeBPS, Reason: c.Reason,
		RequestedBy: c.RequestedBy, RequestedByYou: c.RequestedBy == h.actor(r).ID,
		RequestedAt: c.RequestedAt, ExpiresAt: c.ExpiresAt, Status: string(c.Status),
		DecidedBy: c.DecidedBy, DecidedAt: c.DecidedAt, DecisionReason: c.DecisionReason,
	}
}

func (h *Handler) readBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := httpx.DecodeJSON(r, dst); err != nil {
		httpx.WriteError(w, r, identityhttp.ProblemBadJSON)
		return false
	}
	return true
}

// RequestActivation asks for a pending merchant to go live. A second staff
// member decides.
func (h *Handler) RequestActivation(w http.ResponseWriter, r *http.Request) {
	var req reasonRequest
	if !h.readBody(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	c, err := h.d.Merchants.RequestActivation(r.Context(), id, h.origin(r, req.Reason))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, h.view(r, c))
}

// RequestFees asks for new fees. A second staff member decides.
func (h *Handler) RequestFees(w http.ResponseWriter, r *http.Request) {
	var req feesRequest
	if !h.readBody(w, r, &req) {
		return
	}
	c, err := h.d.Merchants.RequestFeeChange(r.Context(), merchants.FeeChange{
		MerchantID: r.PathValue("id"), FeeBPS: req.FeeBPS, PayoutFeeBPS: req.PayoutFeeBPS,
		Origin: h.origin(r, req.Reason),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, h.view(r, c))
}

// Approvals lists change requests, newest first. Filter by status or merchant.
func (h *Handler) Approvals(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rows, err := h.d.Merchants.ListRequests(r.Context(), merchants.RequestFilter{
		Status: merchants.RequestStatus(q.Get("status")), MerchantID: q.Get("merchant_id"),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := requestsResponse{Data: make([]requestJSON, len(rows))}
	for i, c := range rows {
		out.Data[i] = h.view(r, c)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Approve applies a request. The person who asked cannot approve it.
func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) { h.decide(w, r, true) }

// Reject turns a request down, leaving the merchant as it is.
func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) { h.decide(w, r, false) }

func (h *Handler) decide(w http.ResponseWriter, r *http.Request, approve bool) {
	var req reasonRequest
	if !h.readBody(w, r, &req) {
		return
	}
	c, err := h.d.Merchants.Decide(r.Context(), merchants.Decision{
		RequestID: r.PathValue("id"), Approve: approve, Origin: h.origin(r, req.Reason),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.view(r, c))
}
