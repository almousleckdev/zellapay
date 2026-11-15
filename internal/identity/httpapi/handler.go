// Package httpapi serves the merchant dashboard's account and security
// routes under /dashboard. Only the dashboard's own server calls them; the
// browser never does, so there is no CORS and no cookie here.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/zellapay/zellapay/internal/access"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/identityhttp"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// maxBody bounds every request body. Nothing here needs more.
const maxBody = 16 << 10

// Accounts is what the public account routes need from identity.
type Accounts interface {
	AcceptInvitation(ctx context.Context, in identity.AcceptInput) (identity.User, error)
	ConfirmEmail(ctx context.Context, token string) error
	RequestPasswordReset(ctx context.Context, email string) error
	CompletePasswordReset(ctx context.Context, in identity.ResetInput) error
}

// Sessions is what the session and security routes need.
type Sessions interface {
	Login(ctx context.Context, in identity.LoginInput) (identity.Issued, error)
	CompleteMFA(ctx context.Context, in identity.MFAInput) (identity.Verification, error)
	Resolve(ctx context.Context, token string) (identity.Principal, error)
	Logout(ctx context.Context, token string) error
	BeginMFAEnrolment(ctx context.Context, token string) (identity.Enrolment, error)
	ConfirmMFAEnrolment(ctx context.Context, in identity.MFAInput) ([]string, error)
	List(ctx context.Context, userID string) ([]identity.Session, error)
	End(ctx context.Context, userID, sessionID string) error
	EndOthers(ctx context.Context, userID, keepID string) (int, error)
	ChangePassword(ctx context.Context, in identity.PasswordChangeInput) error
	RegenerateRecoveryCodes(ctx context.Context, token string) ([]string, error)
	RemoveAuthenticator(ctx context.Context, token string) error
}

// Policy decides whether a principal may act.
type Policy interface {
	Check(
		pr identity.Principal, p access.Permission, r access.Resource, now time.Time,
	) error
}

// Deps are the collaborators of Handler.
type Deps struct {
	Accounts Accounts
	Sessions Sessions
	Policy   Policy
	Clock    clock.Clock
	// ClientIP returns the end user's address as the trusted proxy reports it.
	ClientIP func(*http.Request) string
	Log      *slog.Logger
}

// Handler serves the dashboard account routes.
type Handler struct{ d Deps }

// New returns a Handler.
func New(d Deps) *Handler { return &Handler{d: d} }

// decode reads one bounded JSON object into dst, or writes the error.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := httpx.DecodeJSON(r, dst); err != nil {
		httpx.WriteError(w, r, identityhttp.ProblemBadJSON)
		return false
	}
	return true
}

func (h *Handler) source(r *http.Request) identity.Source {
	return identity.Source{IP: h.d.ClientIP(r), UserAgent: truncate(r.UserAgent(), 200)}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
