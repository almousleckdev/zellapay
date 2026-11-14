package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore/sqlcgen"
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
		ID: id, FeeBps: int32(bps), Now: ts(now),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return merchants.ErrNotFound
	}
	return nil
}
