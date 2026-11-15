package httpapi

import "net/http"

// Mount registers the dashboard account routes on mux. Public routes need no
// session. Session routes need a valid session token; the session routes
// that move money or secrets also pass the access policy inside the handler.
func (h *Handler) Mount(mux *http.ServeMux) {
	public := func(pattern string, f http.HandlerFunc) {
		mux.Handle(pattern, NoStore(f))
	}
	session := func(pattern string, f http.HandlerFunc) {
		mux.Handle(pattern, NoStore(h.RequireSession(f)))
	}
	public("POST /dashboard/invitations/accept", h.AcceptInvitation)
	public("POST /dashboard/email/verify", h.VerifyEmail)
	public("POST /dashboard/password/reset-request", h.RequestReset)
	public("POST /dashboard/password/reset", h.CompleteReset)
	public("POST /dashboard/session", h.Login)

	session("GET /dashboard/session", h.Current)
	session("DELETE /dashboard/session", h.Logout)
	session("POST /dashboard/session/mfa", h.VerifyMFA)
	session("POST /dashboard/mfa/enrolment", h.BeginEnrolment)
	session("POST /dashboard/mfa/enrolment/confirm", h.ConfirmEnrolment)
	session("GET /dashboard/sessions", h.ListSessions)
	session("DELETE /dashboard/sessions/{id}", h.EndSession)
	session("POST /dashboard/sessions/end-others", h.EndOtherSessions)
}
