package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/access"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// BeginEnrolment starts setting up the authenticator app.
func (h *Handler) BeginEnrolment(w http.ResponseWriter, r *http.Request) {
	c, _ := callerFrom(r.Context())
	en, err := h.d.Sessions.BeginMFAEnrolment(r.Context(), c.Token)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, enrolmentResponse{ManualKey: en.ManualKey, URI: en.URI})
}

// ConfirmEnrolment finishes setup with the first code and returns the
// recovery codes, which are shown only this once.
func (h *Handler) ConfirmEnrolment(w http.ResponseWriter, r *http.Request) {
	var req codeRequest
	if !h.decode(w, r, &req) {
		return
	}
	c, _ := callerFrom(r.Context())
	codes, err := h.d.Sessions.ConfirmMFAEnrolment(r.Context(), identity.MFAInput{
		Token: c.Token, Code: req.Code, IP: h.d.ClientIP(r),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, recoveryCodesResponse{RecoveryCodes: codes})
}

// allow checks the caller may manage their sessions, or writes the refusal.
func (h *Handler) allow(w http.ResponseWriter, r *http.Request) (caller, bool) {
	c, _ := callerFrom(r.Context())
	err := h.d.Policy.Check(c.Principal, access.SessionsManage, access.Resource{}, h.d.Clock.Now())
	if err != nil {
		h.fail(w, r, err)
		return caller{}, false
	}
	return c, true
}

// ListSessions lists the caller's open sessions.
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	c, ok := h.allow(w, r)
	if !ok {
		return
	}
	list, err := h.d.Sessions.List(r.Context(), c.User.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]sessionJSON, 0, len(list))
	for _, s := range list {
		out = append(out, sessionView(s, c.Session.ID))
	}
	httpx.WriteJSON(w, http.StatusOK, sessionsResponse{Data: out})
}

// EndSession ends one of the caller's sessions.
func (h *Handler) EndSession(w http.ResponseWriter, r *http.Request) {
	c, ok := h.allow(w, r)
	if !ok {
		return
	}
	if err := h.d.Sessions.End(r.Context(), c.User.ID, r.PathValue("id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// EndOtherSessions ends every session of the caller except the current one.
func (h *Handler) EndOtherSessions(w http.ResponseWriter, r *http.Request) {
	c, ok := h.allow(w, r)
	if !ok {
		return
	}
	n, err := h.d.Sessions.EndOthers(r.Context(), c.User.ID, c.Session.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, endedResponse{Ended: n})
}
