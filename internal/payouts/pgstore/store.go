// Package pgstore persists payouts in PostgreSQL. Recipient phone numbers
// are encrypted before they are written and looked up by a keyed index.
package pgstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/payouts/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/crypto"
)

const phoneIndexLabel = "payout.recipient_phone"

// Store implements payouts.Repository on PostgreSQL.
type Store struct {
	pool    *pgxpool.Pool
	sealer  *crypto.Sealer
	indexer *crypto.BlindIndexer
}

var _ payouts.Repository = (*Store)(nil)

// New returns a Store that protects phone numbers with sealer and indexer.
func New(pool *pgxpool.Pool, sealer *crypto.Sealer, indexer *crypto.BlindIndexer) *Store {
	return &Store{pool: pool, sealer: sealer, indexer: indexer}
}

// phoneContext binds a sealed phone number to its payout.
func phoneContext(payoutID string) []byte {
	return []byte("payout:" + payoutID + ":recipient_phone")
}

// Create inserts nc, or returns the earlier payout that used the same
// idempotency key when the request is identical.
func (s *Store) Create(
	ctx context.Context, nc payouts.NewPayout,
) (payouts.Payout, bool, error) {
	sealed, err := s.sealer.Seal([]byte(nc.Phone), phoneContext(nc.ID))
	if err != nil {
		return payouts.Payout{}, false, fmt.Errorf("seal phone: %w", err)
	}
	nc.PhoneIndex = s.indexer.Index(phoneIndexLabel, nc.Phone)
	q := sqlcgen.New(s.pool)
	_, err = q.InsertPayout(ctx, insertParams(nc, sealed))
	if errors.Is(err, pgx.ErrNoRows) {
		return s.existing(ctx, q, nc)
	}
	if err != nil {
		return payouts.Payout{}, false, err
	}
	c, err := s.Get(ctx, nc.ID)
	return c, true, err
}

// existing resolves an idempotency-key collision: the same request returns
// the original, a different one is a conflict.
func (s *Store) existing(
	ctx context.Context, q *sqlcgen.Queries, nc payouts.NewPayout,
) (payouts.Payout, bool, error) {
	row, err := q.GetPayoutByKey(ctx, sqlcgen.GetPayoutByKeyParams{
		MerchantID: nc.MerchantID, IdempotencyKey: nc.IdempotencyKey,
	})
	if err != nil {
		return payouts.Payout{}, false, err
	}
	c := fromRow(sqlcgen.GetPayoutRow(row))
	same := c.Amount == nc.Amount && c.Currency == nc.Currency &&
		c.MerchantReference == nc.MerchantReference &&
		bytes.Equal(c.PhoneIndex, nc.PhoneIndex)
	if !same {
		return payouts.Payout{}, false, payouts.ErrIdempotencyConflict
	}
	return c, false, nil
}

// Get loads a payout by ID.
func (s *Store) Get(ctx context.Context, id string) (payouts.Payout, error) {
	row, err := sqlcgen.New(s.pool).GetPayout(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return payouts.Payout{}, payouts.ErrNotFound
	}
	if err != nil {
		return payouts.Payout{}, err
	}
	return fromRow(row), nil
}
