// Package keysapi serves API key management under /dashboard. Keys are
// minted for the signed-in user's own merchant, never one named by the
// request, and a key can never hold more authority than its creator.
package keysapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/zellapay/zellapay/internal/access"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

const maxBody = 4 << 10

// Keys is what these routes need from the merchants module.
type Keys interface {
	ListKeys(ctx context.Context, merchantID string) ([]merchants.APIKey, error)
	IssueKey(ctx context.Context, spec merchants.KeySpec) (merchants.IssuedKey, error)
	RotateKey(
		ctx context.Context, ref merchants.KeyRef, overlap time.Duration,
	) (merchants.IssuedKey, error)
	RevokeKeyRef(ctx context.Context, ref merchants.KeyRef) error
}

// Policy decides whether a principal may act.
type Policy interface {
	Check(pr identity.Principal, p access.Permission, r access.Resource, now time.Time) error
}

// Deps are the collaborators of Handler.
type Deps struct {
	Keys   Keys
	Policy Policy
	Clock  clock.Clock
	// Mode is the environment this deployment serves; new keys get it.
	Mode merchants.Mode
	// Principal returns the signed-in user, as set by the session guard.
	Principal func(context.Context) (identity.Principal, bool)
	Log       *slog.Logger
}

// Handler serves the key routes.
type Handler struct{ d Deps }

// New returns a Handler.
func New(d Deps) *Handler { return &Handler{d: d} }

// require checks the caller holds the permission, or writes the refusal.
func (h *Handler) require(
	w http.ResponseWriter, r *http.Request, p access.Permission,
) (identity.Principal, bool) {
	pr, ok := h.d.Principal(r.Context())
	if !ok {
		h.fail(w, r, access.ErrForbidden)
		return identity.Principal{}, false
	}
	if err := h.d.Policy.Check(pr, p, access.Resource{}, h.d.Clock.Now()); err != nil {
		h.fail(w, r, err)
		return identity.Principal{}, false
	}
	return pr, true
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := httpx.DecodeJSON(r, dst); err != nil {
		httpx.WriteError(w, r, problemBadJSON)
		return false
	}
	return true
}
