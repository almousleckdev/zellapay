// Package apiserver assembles the public HTTP API from its modules.
package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/zellapay/zellapay/internal/access"
	adminapi "github.com/zellapay/zellapay/internal/admin/httpapi"
	"github.com/zellapay/zellapay/internal/apidocs"
	"github.com/zellapay/zellapay/internal/collections/httpapi"
	identityapi "github.com/zellapay/zellapay/internal/identity/httpapi"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/httpauth"
	"github.com/zellapay/zellapay/internal/merchants/keysapi"
	payoutsapi "github.com/zellapay/zellapay/internal/payouts/httpapi"
	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/platform/idempotency"
	staffapi "github.com/zellapay/zellapay/internal/staff/httpapi"
	webhooksapi "github.com/zellapay/zellapay/internal/webhooks/httpapi"
	"github.com/zellapay/zellapay/internal/wiring"
)

// Options configure the server.
type Options struct {
	DocsEnabled  bool
	ClientIP     httpauth.ClientIP // defaults to the TCP peer address
	MaxBodyBytes int64
	Drain        *httpx.Drain
}

const defaultMaxBody = 1 << 20

// New returns the API handler: health, optional docs, and the versioned
// merchant API behind authentication, scopes and idempotency.
func New(c *wiring.Container, opts Options, log *slog.Logger) http.Handler {
	if opts.ClientIP == nil {
		opts.ClientIP = httpauth.RemoteAddr
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = defaultMaxBody
	}
	mux := http.NewServeMux()
	httpx.Health(mux, opts.Drain.Check(httpx.ReadyTimeout, c.Ready))
	if opts.DocsEnabled {
		apidocs.Register(mux)
	}
	mux.Handle("POST /callbacks/{operator}", c.Callbacks.Handler(log))
	mountCollections(mux, c, opts, log)
	mountPayouts(mux, c, opts, log)
	mountWebhooks(mux, c, opts, log)
	mountDashboard(mux, c, opts, log)
	mountAdmin(mux, c, opts, log)
	return httpx.Harden(log, mux, httpx.BodyLimits{Default: opts.MaxBodyBytes})
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

func mountPayouts(mux *http.ServeMux, c *wiring.Container, opts Options, log *slog.Logger) {
	h := payoutsapi.New(c.Payouts, merchantOf, log)
	h.Mount(mux, payoutsapi.Guards{
		Authenticate: httpauth.Require(c.Merchants, opts.ClientIP, log),
		Read:         httpauth.RequireScope(merchants.ScopeRead),
		Write:        httpauth.RequireScope(merchants.ScopePayoutsWrite),
		Idempotent:   idempotent(c, log),
	})
}

// mountDashboard serves the routes of the merchant dashboard: account and
// security, and API keys. Only the dashboard's own server should call them.
func mountDashboard(mux *http.ServeMux, c *wiring.Container, opts Options, log *slog.Logger) {
	policy := access.NewPolicy(access.DefaultConfig())
	account := identityapi.New(identityapi.Deps{
		Accounts: c.Identity, Sessions: c.Sessions, Policy: policy, Clock: c.Clock,
		ClientIP: func(r *http.Request) string {
			if addr := opts.ClientIP(r); addr.IsValid() {
				return addr.String()
			}
			return ""
		},
		Log: log,
	})
	account.Mount(mux)
	keysapi.New(keysapi.Deps{
		Keys: c.Merchants, Policy: policy, Clock: c.Clock, Mode: c.Settings.Mode,
		Principal: identityapi.PrincipalFrom, Log: log,
	}).Mount(mux, account.RequireSession)
}

// mountAdmin serves the admin console's routes. Only the console's own server
// should call them.
func mountAdmin(mux *http.ServeMux, c *wiring.Container, opts Options, log *slog.Logger) {
	clientIP := func(r *http.Request) string {
		if addr := opts.ClientIP(r); addr.IsValid() {
			return addr.String()
		}
		return ""
	}
	staffapi.New(staffapi.Deps{Staff: c.Staff, ClientIP: clientIP, Log: log}).Mount(mux)
	adminapi.New(adminapi.Deps{
		Sessions: c.Staff, Merchants: c.Merchants, Invites: c.Identity,
		Directory: c.Staff, History: c.Audit, Routing: c.Router,
		ClientIP: clientIP, Log: log,
	}).Mount(mux)
}
