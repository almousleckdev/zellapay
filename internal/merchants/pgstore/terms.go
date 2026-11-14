package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

// FeeBPS returns the merchant's fee in hundredths of a percent.
func (s *Store) FeeBPS(ctx context.Context, id string) (int, error) {
	bps, err := sqlcgen.New(s.pool).GetMerchantFeeBPS(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, merchants.ErrNotFound
	}
	return int(bps), err
}

// SetFeeBPS stores the merchant's fee.
func (s *Store) SetFeeBPS(ctx context.Context, id string, bps int, now time.Time) error {
	if bps < 0 || bps > 10_000 {
		return merchants.ErrInvalidInput
	}
	n, err := sqlcgen.New(s.pool).SetMerchantFeeBPS(ctx, sqlcgen.SetMerchantFeeBPSParams{
		ID: id, FeeBps: pgconv.Int32(bps), Now: ts(now),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return merchants.ErrNotFound
	}
	return nil
}

// PayoutFeeBPS returns the merchant's payout fee in hundredths of a percent.
func (s *Store) PayoutFeeBPS(ctx context.Context, id string) (int, error) {
	bps, err := sqlcgen.New(s.pool).GetMerchantPayoutFeeBPS(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, merchants.ErrNotFound
	}
	return int(bps), err
}

// SetPayoutFeeBPS stores the merchant's payout fee.
func (s *Store) SetPayoutFeeBPS(ctx context.Context, id string, bps int, now time.Time) error {
	if bps < 0 || bps > 10_000 {
		return merchants.ErrInvalidInput
	}
	params := sqlcgen.SetMerchantPayoutFeeBPSParams{
		ID: id, FeeBps: pgconv.Int32(bps), Now: ts(now),
	}
	n, err := sqlcgen.New(s.pool).SetMerchantPayoutFeeBPS(ctx, params)
	if err != nil {
		return err
	}
	if n == 0 {
		return merchants.ErrNotFound
	}
	return nil
}
