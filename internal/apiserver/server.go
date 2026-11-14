// Package apiserver assembles the public HTTP API from its modules.
package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/zellapay/zellapay/internal/apidocs"
	"github.com/zellapay/zellapay/internal/collections/httpapi"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/httpauth"
	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/platform/idempotency"
	webhooksapi "github.com/zellapay/zellapay/internal/webhooks/httpapi"
	"github.com/zellapay/zellapay/internal/wiring"
)

// Options configure the server.
type Options struct {
	DocsEnabled bool
	ClientIP    httpauth.ClientIP // defaults to the TCP peer address
}

// New returns the API handler: health, optional docs, and the versioned
// merchant API behind authentication, scopes and idempotency.
func New(c *wiring.Container, opts Options, log *slog.Logger) http.Handler {
	if opts.ClientIP == nil {
		opts.ClientIP = httpauth.RemoteAddr
	}
	mux := http.NewServeMux()
	httpx.Health(mux, c.Ready)
	if opts.DocsEnabled {
		apidocs.Register(mux)
	}
	mountCollections(mux, c, opts, log)
	mountWebhooks(mux, c, opts, log)
	return httpx.Middleware(log, mux)
}

func mountCollections(mux *http.ServeMux, c *wiring.Container, opts Options, log *slog.Logger) {
	h := httpapi.New(c.Collections, merchantOf, log)
	h.Mount(mux, httpapi.Guards{
		Authenticate: httpauth.Require(c.Merchants, opts.ClientIP, log),
		Read:         httpauth.RequireScope(merchants.ScopeRead),
		Write:        httpauth.RequireScope(merchants.ScopeCollectionsWrite),
		Idempotent:   idempotent(c, log),
	})
}

// merchantOf returns the merchant that authentication placed in the request.
func merchantOf(r *http.Request) (string, bool) {
	p, ok := httpauth.PrincipalFrom(r.Context())
	return p.MerchantID, ok
}

func idempotent(c *wiring.Container, log *slog.Logger) httpx.Decorator {
	return idempotency.Middleware(c.Idempotency, c.Clock, idempotency.Options{
		Scope: merchantOf,
		Log:   log,
	})
}

func mountWebhooks(mux *http.ServeMux, c *wiring.Container, opts Options, log *slog.Logger) {
	h := webhooksapi.New(c.Webhooks, merchantOf, log)
	h.Mount(mux, webhooksapi.Guards{
		Authenticate: httpauth.Require(c.Merchants, opts.ClientIP, log),
		Read:         httpauth.RequireScope(merchants.ScopeRead),
		Write:        httpauth.RequireScope(merchants.ScopeWebhooksWrite),
		Idempotent:   idempotent(c, log),
	})
}
