package merchants

import (
	"context"
	"time"
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
	InsertKey(ctx context.Context, k APIKey) error
	KeyByLookup(ctx context.Context, lookupID string) (StoredKey, error)
	RevokeKey(ctx context.Context, merchantID, keyID string, now time.Time) error
	// RotateKey inserts next and sets the expiry of the key being replaced
	// in one transaction.
	RotateKey(ctx context.Context, old KeyRef, next APIKey, expires time.Time) error
	ListKeys(ctx context.Context, merchantID string) ([]APIKey, error)
}

// KeyRef identifies a key within a merchant.
type KeyRef struct {
	MerchantID string
	KeyID      string
}
