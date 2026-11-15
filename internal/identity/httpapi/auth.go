package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

type ctxKey struct{}

// caller is who the session in the request belongs to.
type caller struct {
	identity.Principal
	Token string
}

func callerFrom(ctx context.Context) (caller, bool) {
	c, ok := ctx.Value(ctxKey{}).(caller)
	return c, ok
}

func bearer(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}

// RequireSession rejects requests without a valid session token and stores
// the caller in the context. A session that has not finished MFA is still
// accepted; each route decides what that allows.
func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearer(r)
		if !ok {
			httpx.WriteError(w, r, problemSession)
			return
		}
		p, err := h.d.Sessions.Resolve(r.Context(), token)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, caller{Principal: p, Token: token})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// NoStore stops anything between the server and the browser from caching a
// response that may hold a session token or a secret.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
