package merchants

import (
	"context"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
)

// StatusUpdate is one guarded status change.
type StatusUpdate struct {
	ID       string
	From, To Status
	At       time.Time
}

// StoredKey is an API key together with its merchant's current status.
type StoredKey struct {
	Key            APIKey
	MerchantStatus Status
}

// Repository is the persistence the service needs.
type Repository interface {
	// CreateMerchant stores m and appends the audit entries in one transaction.
	CreateMerchant(ctx context.Context, m Merchant, fx Effects) (Merchant, error)
	MerchantCounts(ctx context.Context) (map[Status]int, error)
	// CreatedPerDay counts merchants created on each local day from since on.
	// Days with none are left out.
	CreatedPerDay(ctx context.Context, since time.Time) ([]DayCount, error)
	GetMerchant(ctx context.Context, id string) (Merchant, error)
	SetStatus(ctx context.Context, id string, s Status, now time.Time) error
	ListMerchants(ctx context.Context, q ListQuery) ([]Merchant, error)
	// ChangeStatus moves a merchant from one status to another and appends
	// the audit entries in one transaction. It returns ErrNotFound when there
	// is no such merchant and ErrInvalidTransition when it is not in From.
	ChangeStatus(ctx context.Context, u StatusUpdate, fx Effects) error
	// CreateChangeRequest stores r and its audit entries. It first expires any
	// stale open request of the same kind, and returns ErrRequestOpen if a
	// live one remains.
	CreateChangeRequest(ctx context.Context, r ChangeRequest, fx Effects) (ChangeRequest, error)
	// DecideChangeRequest locks the request and asks fn what to do. An approved
	// outcome applies the change and a failure leaves everything as it was.
	DecideChangeRequest(ctx context.Context, id string, fn DecideFunc) (ChangeRequest, error)
	ListChangeRequests(ctx context.Context, f RequestFilter) ([]ChangeRequest, error)
	CountPendingChangeRequests(ctx context.Context, now time.Time) (int, error)
	FeeBPS(ctx context.Context, id string) (int, error)
	PayoutFeeBPS(ctx context.Context, id string) (int, error)
	SetPayoutFeeBPS(ctx context.Context, id string, bps int, now time.Time) error
	SetFeeBPS(ctx context.Context, id string, bps int, now time.Time) error
	InsertKey(ctx context.Context, k APIKey, fx Effects) error
	KeyByLookup(ctx context.Context, lookupID string) (StoredKey, error)
	RevokeKey(ctx context.Context, ref KeyRef, now time.Time, fx Effects) error
	// RotateKey inserts the new key and sets the expiry of the key being
	// replaced in one transaction.
	RotateKey(ctx context.Context, r Rotation, fx Effects) error
	ListKeys(ctx context.Context, merchantID string) ([]APIKey, error)
}

// KeyRef identifies a key within a merchant. By, when set, is who is acting,
// and makes the change audited.
type KeyRef struct {
	MerchantID string
	KeyID      string
	By         *audit.Actor
}

// Rotation replaces Old with Next, letting Old work until OldExpires.
type Rotation struct {
	Old        KeyRef
	Next       APIKey
	OldExpires time.Time
}

// Effects are records written in the same transaction as a key change.
type Effects struct{ Audit []audit.Pending }
