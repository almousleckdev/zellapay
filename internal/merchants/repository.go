package merchants

import (
	"context"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
)

// StoredKey is an API key together with its merchant's current status.
type StoredKey struct {
	Key            APIKey
	MerchantStatus Status
}

// Repository is the persistence the service needs.
type Repository interface {
	CreateMerchant(ctx context.Context, m Merchant) (Merchant, error)
	GetMerchant(ctx context.Context, id string) (Merchant, error)
	SetStatus(ctx context.Context, id string, s Status, now time.Time) error
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
