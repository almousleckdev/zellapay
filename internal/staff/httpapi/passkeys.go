package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/staff"
)

// BeginRegistration returns the options the browser needs to create a passkey.
func (h *Handler) BeginRegistration(w http.ResponseWriter, r *http.Request) {
	options, err := h.d.Staff.BeginRegistration(r.Context(), callerFrom(r.Context()).Token)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, options)
}

// FinishRegistration stores the passkey the browser created.
func (h *Handler) FinishRegistration(w http.ResponseWriter, r *http.Request) {
	var req ceremonyRequest
	if !h.decode(w, r, maxCeremonyBody, &req) {
		return
	}
	key, err := h.d.Staff.FinishRegistration(r.Context(), staff.FinishInput{
		Token: callerFrom(r.Context()).Token, Body: req.Credential, Label: req.Label,
		IP: h.d.ClientIP(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, passkeyView(key))
}

// BeginLogin returns the challenge the browser's passkey must sign.
func (h *Handler) BeginLogin(w http.ResponseWriter, r *http.Request) {
	options, err := h.d.Staff.BeginLogin(r.Context(), callerFrom(r.Context()).Token)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, options)
}

// FinishLogin checks the passkey's signed answer.
func (h *Handler) FinishLogin(w http.ResponseWriter, r *http.Request) {
	var req ceremonyRequest
	if !h.decode(w, r, maxCeremonyBody, &req) {
		return
	}
	err := h.d.Staff.FinishLogin(r.Context(), staff.FinishInput{
		Token: callerFrom(r.Context()).Token, Body: req.Credential, IP: h.d.ClientIP(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// List returns the member's passkeys.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.d.Staff.ListPasskeys(r.Context(), callerFrom(r.Context()).Token)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]passkeyJSON, 0, len(keys))
	for _, k := range keys {
		out = append(out, passkeyView(k))
	}
	httpx.WriteJSON(w, http.StatusOK, passkeysResponse{Data: out})
}
