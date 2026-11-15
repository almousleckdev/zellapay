package httpapi

import "net/http"

// Mount registers the admin routes on mux. Signing in needs no session; every
// other route needs one.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.Handle("POST /admin/session", noStore(http.HandlerFunc(h.Login)))
	session := func(pattern string, f http.HandlerFunc) {
		mux.Handle(pattern, noStore(h.RequireSession(f)))
	}
	session("GET /admin/session", h.Current)
	session("DELETE /admin/session", h.Logout)
	session("POST /admin/session/passkey/begin", h.BeginLogin)
	session("POST /admin/session/passkey/finish", h.FinishLogin)
	session("GET /admin/passkeys", h.List)
	session("POST /admin/passkeys/begin", h.BeginRegistration)
	session("POST /admin/passkeys/finish", h.FinishRegistration)
}
