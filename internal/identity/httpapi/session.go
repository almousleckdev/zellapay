package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Login checks the password and opens a session. The caller still has to
// pass the second factor, which the response says.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !h.decode(w, r, &req) {
		return
	}
	issued, err := h.d.Sessions.Login(r.Context(), identity.LoginInput{
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

// Current describes the signed-in user and what their session still needs.
func (h *Handler) Current(w http.ResponseWriter, r *http.Request) {
	c, _ := callerFrom(r.Context())
	httpx.WriteJSON(w, http.StatusOK, currentResponse{
		User: userView(c.User), Session: sessionView(c.Session, c.Session.ID), MFA: string(c.MFA),
	})
}

// Logout ends the current session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	c, _ := callerFrom(r.Context())
	if err := h.d.Sessions.Logout(r.Context(), c.Token); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// VerifyMFA proves the second factor for the current session.
func (h *Handler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	var req codeRequest
	if !h.decode(w, r, &req) {
		return
	}
	c, _ := callerFrom(r.Context())
	v, err := h.d.Sessions.CompleteMFA(r.Context(), identity.MFAInput{
		Token: c.Token, Code: req.Code, IP: h.d.ClientIP(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, mfaResponse{
		RecoveryUsed: v.RecoveryUsed, CodesLeft: v.CodesLeft,
	})
}
