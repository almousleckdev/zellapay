// Package pgstore persists merchants and API keys in PostgreSQL.
package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

var (
	ts      = pgconv.Timestamptz
	timePtr = pgconv.TimePtr
)

// Store implements merchants.Repository on PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

var _ merchants.Repository = (*Store)(nil)

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// CreateMerchant inserts m and returns the stored row.
func (s *Store) CreateMerchant(
	ctx context.Context, m merchants.Merchant,
) (merchants.Merchant, error) {
	row, err := sqlcgen.New(s.pool).CreateMerchant(ctx, sqlcgen.CreateMerchantParams{
		ID:              m.ID,
		Name:            m.Name,
		DefaultCurrency: m.DefaultCurrency,
		Now:             ts(m.CreatedAt),
	})
	if err != nil {
		return merchants.Merchant{}, fmt.Errorf("create merchant: %w", err)
	}
	return merchantFromRow(sqlcgen.GetMerchantRow(row)), nil
}

// GetMerchant loads a merchant by ID.
func (s *Store) GetMerchant(ctx context.Context, id string) (merchants.Merchant, error) {
	row, err := sqlcgen.New(s.pool).GetMerchant(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return merchants.Merchant{}, merchants.ErrNotFound
	}
	if err != nil {
		return merchants.Merchant{}, err
	}
	return merchantFromRow(row), nil
}

// SetStatus changes a merchant's lifecycle state.
func (s *Store) SetStatus(
	ctx context.Context, id string, st merchants.Status, now time.Time,
) error {
	n, err := sqlcgen.New(s.pool).SetMerchantStatus(ctx, sqlcgen.SetMerchantStatusParams{
		ID: id, Status: string(st), Now: ts(now),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return merchants.ErrNotFound
	}
	return nil
}

func merchantFromRow(r sqlcgen.GetMerchantRow) merchants.Merchant {
	return merchants.Merchant{
		ID:              r.ID,
		Name:            r.Name,
		Status:          merchants.Status(r.Status),
		KYCStatus:       r.KycStatus,
		DefaultCurrency: r.DefaultCurrency,
		CreatedAt:       r.CreatedAt.Time,
	}
}
