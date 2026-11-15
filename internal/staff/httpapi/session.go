package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/staff"
)

func (h *Handler) source(r *http.Request) identity.Source {
	ua := r.UserAgent()
	if len(ua) > 200 {
		ua = ua[:200]
	}
	return identity.Source{IP: h.d.ClientIP(r), UserAgent: ua}
}

// Login checks the password and opens a session that still needs a passkey.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !h.decode(w, r, maxBody, &req) {
		return
	}
	issued, err := h.d.Staff.Login(r.Context(), staff.LoginInput{
		Email: req.Email, Password: req.Password, Source: h.source(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, loginResponse{
		SessionToken: issued.Token, ExpiresAt: issued.Session.ExpiresAt, MFA: string(issued.MFA),
	})
}

// Current describes the signed-in member and what their session still needs.
func (h *Handler) Current(w http.ResponseWriter, r *http.Request) {
	c := callerFrom(r.Context())
	httpx.WriteJSON(w, http.StatusOK, currentResponse{
		User: userJSON{
			ID: c.User.ID, Email: c.User.Email, Name: c.User.Name, Role: string(c.User.Role),
		},
		Session: sessionJSON{
			ID: c.Session.ID, CreatedAt: c.Session.CreatedAt, ExpiresAt: c.Session.ExpiresAt,
			MFAVerifiedAt: c.Session.MFAAt,
		},
		MFA: string(c.MFA), Passkeys: c.Passkeys, Permissions: permissionsOf(c.User.Role),
	})
}

// Logout ends the current session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.d.Staff.Logout(r.Context(), callerFrom(r.Context()).Token); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
