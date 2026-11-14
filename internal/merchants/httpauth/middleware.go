// Package httpauth authenticates merchant requests by API key.
package httpauth

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Authenticator verifies a raw API key presented from ip.
type Authenticator interface {
	Authenticate(ctx context.Context, raw string, ip netip.Addr) (merchants.Principal, error)
}

// ClientIP extracts the caller's address. Behind a load balancer, supply
// a function that reads the trusted forwarded header instead.
type ClientIP func(*http.Request) netip.Addr

// RemoteAddr returns the TCP peer address of the request.
func RemoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, _ := netip.ParseAddr(host)
	return addr
}

type ctxKey struct{}

// PrincipalFrom returns the authenticated caller stored in ctx.
func PrincipalFrom(ctx context.Context) (merchants.Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(merchants.Principal)
	return p, ok
}

// Require rejects requests without a valid API key and stores the
// authenticated Principal in the request context.
func Require(a Authenticator, ip ClientIP, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				deny(w, r, errMissing)
				return
			}
			p, err := a.Authenticate(r.Context(), raw, ip(r))
			if err != nil {
				log.Warn("authentication failed",
					"request_id", httpx.RequestID(r.Context()), "reason", err.Error())
				deny(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

var errMissing = errors.New("missing credentials")

// bearerToken reads the key from "Authorization: Bearer <key>".
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	token = strings.TrimSpace(token)
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}
