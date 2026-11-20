package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/identityhttp"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

func (h *Handler) actor(r *http.Request) audit.Actor {
	return audit.Actor{Kind: audit.ActorStaff, ID: callerFrom(r.Context()).User.ID}
}

// CreateMerchant registers a merchant in the pending state. A reason is
// required. The merchant cannot take payments until it is activated.
func (h *Handler) CreateMerchant(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, identityhttp.ProblemBadJSON)
		return
	}
	m, err := h.d.Merchants.Create(r.Context(), merchants.NewMerchant{
		Name: req.Name, Currency: req.Currency, By: h.actor(r),
		Reason: req.Reason, Source: h.source(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, merchantView(m))
}

// Invite emails a single-use link that lets one person join the merchant.
func (h *Handler) Invite(w http.ResponseWriter, r *http.Request) {
	var req inviteRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, identityhttp.ProblemBadJSON)
		return
	}
	inv, err := h.d.Invites.Invite(r.Context(), identity.InviteInput{
		MerchantID: r.PathValue("id"), Email: req.Email, Role: identity.Role(req.Role),
		Locale: req.Locale, InvitedBy: h.actor(r), Reason: req.Reason, Source: h.source(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, invitationJSON{
		ID: inv.ID, Email: inv.Email, Role: string(inv.Role), ExpiresAt: inv.ExpiresAt,
	})
}
