package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// AcceptInvitation creates the account an invitation link allows.
func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req acceptRequest
	if !h.decode(w, r, &req) {
		return
	}
	u, err := h.d.Accounts.AcceptInvitation(r.Context(), identity.AcceptInput{
		Token: req.Token, Name: req.Name, Password: req.Password,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, userView(u))
}

// VerifyEmail confirms the address behind a verification link.
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if !h.decode(w, r, &req) {
		return
	}
	if err := h.d.Accounts.ConfirmEmail(r.Context(), req.Token); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RequestReset emails a reset link if the address has an account. It answers
// the same way either way.
func (h *Handler) RequestReset(w http.ResponseWriter, r *http.Request) {
	var req emailRequest
	if !h.decode(w, r, &req) {
		return
	}
	if err := h.d.Accounts.RequestPasswordReset(r.Context(), req.Email); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// CompleteReset sets a new password from a reset link and ends all sessions.
func (h *Handler) CompleteReset(w http.ResponseWriter, r *http.Request) {
	var req resetRequest
	if !h.decode(w, r, &req) {
		return
	}
	err := h.d.Accounts.CompletePasswordReset(r.Context(), identity.ResetInput{
		Token: req.Token, NewPassword: req.NewPassword,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
