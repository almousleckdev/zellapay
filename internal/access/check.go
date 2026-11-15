package access

import (
	"errors"
	"time"

	"github.com/zellapay/zellapay/internal/identity"
)

// Reasons a request is refused. Handlers map them to responses.
var (
	// ErrForbidden means the user's role does not allow this, or the
	// resource belongs to another merchant. The two look the same.
	ErrForbidden = errors.New("access: forbidden")
	// ErrMFARequired means the session has not proven the second factor.
	ErrMFARequired = errors.New("access: second factor required")
	// ErrStepUpRequired means the action needs a recent second factor.
	ErrStepUpRequired = errors.New("access: recent second factor required")
)

// Config sets the step-up rules.
type Config struct {
	// StepUpWindow is how recent the second factor must be for sensitive
	// actions.
	StepUpWindow time.Duration
	// PayoutStepUp is the payout amount, in the smallest unit, from which a
	// recent second factor is required. Zero means every payout.
	PayoutStepUp int64
}

// DefaultConfig returns the design's window and a starting payout threshold.
func DefaultConfig() Config {
	return Config{StepUpWindow: 10 * time.Minute, PayoutStepUp: 100_000}
}

// Resource describes what the user is acting on.
type Resource struct {
	// MerchantID is the owner of the resource, when the action targets one.
	MerchantID string
	// Amount is the money moved, for actions where it matters.
	Amount int64
}

// Policy answers access questions.
type Policy struct{ cfg Config }

// NewPolicy returns a Policy.
func NewPolicy(cfg Config) Policy { return Policy{cfg: cfg} }

// Check returns nil when the principal may perform p on r at now. The order
// matters: MFA, then role, then tenancy, then step-up, so a refusal never
// reveals more than the caller is entitled to know.
func (pol Policy) Check(
	pr identity.Principal, p Permission, r Resource, now time.Time,
) error {
	if pr.MFA != identity.MFADone && !beforeMFA[p] {
		return ErrMFARequired
	}
	if !Allows(pr.User.Role, p) {
		return ErrForbidden
	}
	if r.MerchantID != "" && r.MerchantID != pr.User.MerchantID {
		return ErrForbidden
	}
	if pol.needsStepUp(p, r) && !pr.Session.MFAFresh(now, pol.cfg.StepUpWindow) {
		return ErrStepUpRequired
	}
	return nil
}

func (pol Policy) needsStepUp(p Permission, r Resource) bool {
	if stepUp[p] {
		return true
	}
	return p == PayoutsCreate && r.Amount >= pol.cfg.PayoutStepUp
}
