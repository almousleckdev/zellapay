// Package httpapi serves the admin console's merchant routes under
// /admin/merchants. Only the console's own server calls them.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/zellapay/zellapay/internal/access"
	"github.com/zellapay/zellapay/internal/admin"
	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/identityhttp"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/operators/router"
	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/staff"
)

const maxBody = 4 << 10

// Sessions resolves a session token to the staff member it belongs to.
type Sessions interface {
	Resolve(ctx context.Context, token string) (staff.Principal, error)
}

// Merchants is what these routes need from the merchants module.
type Merchants interface {
	List(ctx context.Context, q merchants.ListQuery) (merchants.Page, error)
	Detail(ctx context.Context, id string) (merchants.Detail, error)
	Create(ctx context.Context, n merchants.NewMerchant) (merchants.Merchant, error)
	Overview(ctx context.Context, days int) (merchants.Overview, error)
	RequestActivation(
		ctx context.Context, id string, o merchants.Origin,
	) (merchants.ChangeRequest, error)
	RequestFeeChange(ctx context.Context, c merchants.FeeChange) (merchants.ChangeRequest, error)
	Decide(ctx context.Context, d merchants.Decision) (merchants.ChangeRequest, error)
	ListRequests(ctx context.Context, f merchants.RequestFilter) ([]merchants.ChangeRequest, error)
	PendingRequestCount(ctx context.Context) (int, error)
	Suspend(ctx context.Context, c merchants.StatusChange) (merchants.Merchant, error)
	Restore(ctx context.Context, c merchants.StatusChange) (merchants.Merchant, error)
}

// Invites sends the invitation that lets a person join a merchant.
type Invites interface {
	Invite(ctx context.Context, in identity.InviteInput) (identity.Invitation, error)
}

// Directory names the staff members behind actions.
type Directory interface {
	Names(ctx context.Context, ids []string) map[string]string
}

// History reads the audit log.
type History interface {
	List(ctx context.Context, f audit.Filter) ([]audit.Record, error)
}

// Rules lists how phone numbers are routed to operators.
type Rules interface{ Rules() []router.Rule }

// Deps are the collaborators of Handler.
type Deps struct {
	Sessions  Sessions
	Merchants Merchants
	Invites   Invites
	Directory Directory
	History   History
	Routing   Rules
	ClientIP  func(*http.Request) string
	Log       *slog.Logger
}

// Handler serves the admin merchant routes.
type Handler struct{ d Deps }

// New returns a Handler.
func New(d Deps) *Handler { return &Handler{d: d} }

// Mount registers the routes on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	route := func(pattern string, p admin.Permission, f http.HandlerFunc) {
		mux.Handle(pattern, noStore(h.guard(p, f)))
	}
	route("GET /admin/merchants", admin.MerchantsRead, h.List)
	route("POST /admin/merchants", admin.MerchantsCreate, h.CreateMerchant)
	route("GET /admin/overview", admin.MerchantsRead, h.Overview)
	route("GET /admin/operators", admin.MerchantsRead, h.Operators)
	route("GET /admin/merchants/{id}/activity", admin.MerchantsRead, h.Activity)
	route("POST /admin/merchants/{id}/activation", admin.MerchantsActivate, h.RequestActivation)
	route("POST /admin/merchants/{id}/fees", admin.MerchantsFees, h.RequestFees)
	route("GET /admin/approvals", admin.MerchantsRead, h.Approvals)
	route("POST /admin/approvals/{id}/approve", admin.ApprovalsDecide, h.Approve)
	route("POST /admin/approvals/{id}/reject", admin.ApprovalsDecide, h.Reject)
	route("GET /admin/merchants/{id}", admin.MerchantsRead, h.Get)
	route("POST /admin/merchants/{id}/invitations", admin.MerchantsInvite, h.Invite)
	route("POST /admin/merchants/{id}/suspend", admin.MerchantsSuspend, h.Suspend)
	route("POST /admin/merchants/{id}/restore", admin.MerchantsRestore, h.Restore)
}

type ctxKey struct{}

func callerFrom(ctx context.Context) staff.Principal {
	p, _ := ctx.Value(ctxKey{}).(staff.Principal)
	return p
}

// guard needs a session that has used its passkey and a role holding p.
func (h *Handler) guard(p admin.Permission, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := httpx.BearerToken(r)
		if !ok {
			httpx.WriteError(w, r, identityhttp.ProblemSession)
			return
		}
		pr, err := h.d.Sessions.Resolve(r.Context(), token)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		if pr.MFA != staff.MFADone {
			h.fail(w, r, access.ErrMFARequired)
			return
		}
		if !admin.Can(pr.User.Role, p) {
			h.fail(w, r, access.ErrForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, pr)
		next(w, r.WithContext(ctx))
	})
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	if p, ok := merchantProblem(err); ok {
		httpx.WriteError(w, r, p)
		return
	}
	identityhttp.Write(w, r, h.d.Log, err)
}

func merchantProblem(err error) (httpx.Problem, bool) {
	switch {
	case errors.Is(err, merchants.ErrNotFound), errors.Is(err, identity.ErrNotFound):
		return httpx.Problem{Status: http.StatusNotFound, Code: "not_found",
			Message: "No such merchant."}, true
	case errors.Is(err, merchants.ErrSelfApproval):
		return httpx.Problem{Status: http.StatusForbidden, Code: "self_approval",
			Message: "A second staff member must decide this request."}, true
	case errors.Is(err, merchants.ErrRequestClosed), errors.Is(err, merchants.ErrRequestOpen):
		return httpx.Problem{Status: http.StatusConflict, Code: "request_conflict",
			Message: "The request is already decided, expired, or already open."}, true
	case errors.Is(err, merchants.ErrInvalidTransition):
		return httpx.Problem{Status: http.StatusConflict, Code: "invalid_transition",
			Message: "The merchant is not in a state this change applies to."}, true
	case errors.Is(err, merchants.ErrInvalidInput), errors.Is(err, audit.ErrInvalidEntry):
		return httpx.Problem{Status: http.StatusBadRequest, Code: "invalid_request",
			Message: "The request is invalid. A reason of at least 5 characters is required."}, true
	}
	return httpx.Problem{}, false
}

func (h *Handler) source(r *http.Request) audit.Source {
	ua := r.UserAgent()
	if len(ua) > 200 {
		ua = ua[:200]
	}
	return audit.Source{IP: h.d.ClientIP(r), UserAgent: ua, RequestID: httpx.RequestID(r.Context())}
}
