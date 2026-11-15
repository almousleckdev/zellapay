package httpapi

import (
	"context"
	"net/http"

	"github.com/zellapay/zellapay/internal/identity/identityhttp"
	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/staff"
)

type ctxKey struct{}

// caller is who the session in the request belongs to.
type caller struct {
	staff.Principal
	Token string
}

func callerFrom(ctx context.Context) caller {
	c, _ := ctx.Value(ctxKey{}).(caller)
	return c
}

// RequireSession rejects requests without a valid session token and stores
// the caller in the context. A session that has not used a passkey is still
// accepted; each route decides what that allows.
func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := httpx.BearerToken(r)
		if !ok {
			httpx.WriteError(w, r, identityhttp.ProblemSession)
			return
		}
		p, err := h.d.Staff.Resolve(r.Context(), token)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, caller{Principal: p, Token: token})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// noStore stops anything from caching a response that may hold a token.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
