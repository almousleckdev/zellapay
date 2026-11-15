package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore/sqlcgen"
)

// InsertKey stores a new API key and its audit entries.
func (s *Store) InsertKey(ctx context.Context, k merchants.APIKey, fx merchants.Effects) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error { return insertKey(ctx, q, k) })
}

// inTx runs work, then appends the audit entries, in one transaction.
func (s *Store) inTx(
	ctx context.Context, fx merchants.Effects, work func(q *sqlcgen.Queries) error,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := work(sqlcgen.New(tx)); err != nil {
			return err
		}
		for _, e := range fx.Audit {
			if s.audit == nil {
				return errors.New("pgstore: audit entries given but no audit log configured")
			}
			if _, err := s.audit.AppendIn(ctx, tx, e); err != nil {
				return fmt.Errorf("append audit entry: %w", err)
			}
		}
		return nil
	})
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
		Label:       k.Label,
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
			r.IpAllowlist, r.CreatedAt, r.ExpiresAt, r.RevokedAt, r.Label,
		}),
		MerchantStatus: merchants.Status(r.MerchantStatus),
	}, nil
}

// RevokeKey permanently disables a key owned by the merchant in ref.
func (s *Store) RevokeKey(
	ctx context.Context, ref merchants.KeyRef, now time.Time, fx merchants.Effects,
) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		n, err := q.RevokeAPIKey(ctx, sqlcgen.RevokeAPIKeyParams{
			ID: ref.KeyID, MerchantID: ref.MerchantID, Now: ts(now),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return merchants.ErrNotFound
		}
		return nil
	})
}

// RotateKey stores the new key and expires the one it replaces, atomically.
func (s *Store) RotateKey(ctx context.Context, r merchants.Rotation, fx merchants.Effects) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		n, err := q.SetAPIKeyExpiry(ctx, sqlcgen.SetAPIKeyExpiryParams{
			ID: r.Old.KeyID, MerchantID: r.Old.MerchantID, ExpiresAt: ts(r.OldExpires),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return merchants.ErrNotFound
		}
		return insertKey(ctx, q, r.Next)
	})
}

func optTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return ts(*t)
}
