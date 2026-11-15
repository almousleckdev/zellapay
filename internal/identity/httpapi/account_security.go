package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/access"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// ChangePassword sets a new password after checking the current one, and
// ends the caller's other sessions. It needs a recent second factor.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	c, ok := h.require(w, r, access.AccountManage)
	if !ok {
		return
	}
	var req changePasswordRequest
	if !h.decode(w, r, &req) {
		return
	}
	err := h.d.Sessions.ChangePassword(r.Context(), identity.PasswordChangeInput{
		Token: c.Token, Current: req.CurrentPassword, New: req.NewPassword, IP: h.d.ClientIP(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegenerateRecoveryCodes replaces the caller's recovery codes and returns
// the new ones, once. It needs a recent second factor.
func (h *Handler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	c, ok := h.require(w, r, access.AccountManage)
	if !ok {
		return
	}
	codes, err := h.d.Sessions.RegenerateRecoveryCodes(r.Context(), c.Token)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, recoveryCodesResponse{RecoveryCodes: codes})
}

// RemoveAuthenticator deletes the caller's authenticator and ends their other
// sessions. The caller must enrol a new one. It needs a recent second factor.
func (h *Handler) RemoveAuthenticator(w http.ResponseWriter, r *http.Request) {
	c, ok := h.require(w, r, access.AccountManage)
	if !ok {
		return
	}
	if err := h.d.Sessions.RemoveAuthenticator(r.Context(), c.Token); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
