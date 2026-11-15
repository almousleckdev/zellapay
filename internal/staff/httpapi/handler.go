// Package httpapi serves the admin console's routes under /admin. Only the
// console's own server calls them; the browser never does.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/zellapay/zellapay/internal/identity/identityhttp"
	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/staff"
)

const (
	maxBody = 16 << 10
	// A passkey answer carries a certificate chain and can be larger.
	maxCeremonyBody = 64 << 10
)

// Staff is what these routes need from the staff module.
type Staff interface {
	Login(ctx context.Context, in staff.LoginInput) (staff.Issued, error)
	Resolve(ctx context.Context, token string) (staff.Principal, error)
	Logout(ctx context.Context, token string) error
	BeginRegistration(ctx context.Context, token string) (staff.RegistrationOptions, error)
	FinishRegistration(ctx context.Context, in staff.FinishInput) (staff.Passkey, error)
	BeginLogin(ctx context.Context, token string) (json.RawMessage, error)
	FinishLogin(ctx context.Context, in staff.FinishInput) error
	ListPasskeys(ctx context.Context, token string) ([]staff.Passkey, error)
}

// Deps are the collaborators of Handler.
type Deps struct {
	Staff Staff
	// ClientIP returns the end user's address as the trusted proxy reports it.
	ClientIP func(*http.Request) string
	Log      *slog.Logger
}

// Handler serves the admin routes.
type Handler struct{ d Deps }

// New returns a Handler.
func New(d Deps) *Handler { return &Handler{d: d} }

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	identityhttp.Write(w, r, h.d.Log, err)
}

// decode reads one bounded JSON object into dst, or writes the error.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, limit int64, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := httpx.DecodeJSON(r, dst); err != nil {
		httpx.WriteError(w, r, identityhttp.ProblemBadJSON)
		return false
	}
	return true
}
