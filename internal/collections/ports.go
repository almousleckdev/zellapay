package collections

import (
	"context"
	"time"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/operators"
)

// UpdateFunc computes the next version of a collection from its current one.
// It runs while the row is locked and must be pure: no I/O. An Event with an
// empty Type is not recorded. Return ErrNoChange to leave the collection
// untouched.
type UpdateFunc func(current Collection) (Collection, Event, error)

// Repository stores collections. The service depends on this interface; the
// PostgreSQL implementation lives in pgstore.
type Repository interface {
	// Create inserts nc. If the merchant already used the idempotency key it
	// returns the existing collection and false, or ErrIdempotencyConflict
	// when the earlier request differed.
	Create(ctx context.Context, nc NewCollection) (Collection, bool, error)
	Get(ctx context.Context, id string) (Collection, error)
	// Update locks the collection, applies fn and stores the result and the
	// event atomically.
	Update(ctx context.Context, id string, fn UpdateFunc) (Collection, error)
	// Record appends an event without changing the collection.
	Record(ctx context.Context, id string, ev Event) error
	// Due returns open collections whose next check time has passed.
	Due(ctx context.Context, now time.Time, limit int) ([]Collection, error)
}

// MerchantTerms provides a merchant's commercial terms.
type MerchantTerms interface {
	FeeBPS(ctx context.Context, merchantID string) (int, error)
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
