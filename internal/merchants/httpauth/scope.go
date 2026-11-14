package httpauth

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// RequireScope rejects authenticated callers whose key lacks scope. It
// must run after Require.
func RequireScope(scope merchants.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFrom(r.Context())
			if !ok {
				deny(w, r, errMissing)
				return
			}
			if !p.Has(scope) {
				httpx.WriteError(w, r, problemScope)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
