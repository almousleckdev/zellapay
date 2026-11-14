package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore/sqlcgen"
)

// InsertKey stores a new API key.
func (s *Store) InsertKey(ctx context.Context, k merchants.APIKey) error {
	return insertKey(ctx, sqlcgen.New(s.pool), k)
}

func insertKey(ctx context.Context, q *sqlcgen.Queries, k merchants.APIKey) error {
	return q.InsertAPIKey(ctx, sqlcgen.InsertAPIKeyParams{
		ID:          k.ID,
		MerchantID:  k.MerchantID,
		LookupID:    k.LookupID,
		SecretHash:  k.SecretHash,
		Mode:        string(k.Mode),
		Scopes:      scopeStrings(k.Scopes),
		IpAllowlist: nonNil(k.IPAllowlist),
		CreatedAt:   ts(k.CreatedAt),
		ExpiresAt:   optTime(k.ExpiresAt),
	})
}

// KeyByLookup finds a key by its public lookup identifier.
func (s *Store) KeyByLookup(ctx context.Context, lookup string) (merchants.StoredKey, error) {
	r, err := sqlcgen.New(s.pool).GetAPIKeyByLookup(ctx, lookup)
	if errors.Is(err, pgx.ErrNoRows) {
		return merchants.StoredKey{}, merchants.ErrNotFound
	}
	if err != nil {
		return merchants.StoredKey{}, err
	}
	return merchants.StoredKey{
		Key: keyFromRow(keyRow{
			r.ID, r.MerchantID, r.LookupID, r.SecretHash, r.Mode, r.Scopes,
			r.IpAllowlist, r.CreatedAt, r.ExpiresAt, r.RevokedAt,
		}),
		MerchantStatus: merchants.Status(r.MerchantStatus),
	}, nil
}

// RevokeKey permanently disables a key owned by merchantID.
func (s *Store) RevokeKey(ctx context.Context, merchantID, keyID string, now time.Time) error {
	n, err := sqlcgen.New(s.pool).RevokeAPIKey(ctx, sqlcgen.RevokeAPIKeyParams{
		ID: keyID, MerchantID: merchantID, Now: ts(now),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return merchants.ErrNotFound
	}
	return nil
}

// RotateKey stores next and expires the key it replaces, atomically.
func (s *Store) RotateKey(
	ctx context.Context, old merchants.KeyRef, next merchants.APIKey, expires time.Time,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		n, err := q.SetAPIKeyExpiry(ctx, sqlcgen.SetAPIKeyExpiryParams{
			ID: old.KeyID, MerchantID: old.MerchantID, ExpiresAt: ts(expires),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return merchants.ErrNotFound
		}
		return insertKey(ctx, q, next)
	})
}

func optTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return ts(*t)
}
