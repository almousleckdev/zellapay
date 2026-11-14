package payouts

import (
	"context"
	"time"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/operators"
)

// UpdateFunc computes the next version of a payout from its current one. It
// runs while the row is locked and must be pure. An Event with an empty Type
// is not recorded. Return ErrNoChange to leave the payout untouched.
type UpdateFunc func(current Payout) (Payout, Event, error)

// OutboxHandler processes a batch of outbox entries. Returning an error
// leaves them unpublished so they are offered again.
type OutboxHandler func(ctx context.Context, entries []OutboxEntry) error

// Repository stores payouts.
type Repository interface {
	// Create inserts np. If the merchant already used the idempotency key it
	// returns the existing payout and false, or ErrIdempotencyConflict when
	// the earlier request differed.
	Create(ctx context.Context, np NewPayout) (Payout, bool, error)
	Get(ctx context.Context, id string) (Payout, error)
	Update(ctx context.Context, id string, fn UpdateFunc) (Payout, error)
	Record(ctx context.Context, id string, ev Event) error
	Due(ctx context.Context, now time.Time, limit int) ([]Payout, error)
	List(ctx context.Context, q ListQuery) ([]Payout, error)
	ClaimOutbox(ctx context.Context, now time.Time, limit int, handle OutboxHandler) (int, error)
}

// MerchantTerms provides a merchant's commercial terms.
type MerchantTerms interface {
	PayoutFeeBPS(ctx context.Context, merchantID string) (int, error)
}

// OperatorRouter finds adapters.
type OperatorRouter interface {
	Route(phone string) (operators.Adapter, error)
	Adapter(code string) (operators.Adapter, bool)
}

// AccountEnsurer creates ledger accounts on demand.
type AccountEnsurer interface {
	EnsureAccount(ctx context.Context, a ledger.Account) error
}
