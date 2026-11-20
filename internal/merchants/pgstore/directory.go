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

// ListMerchants returns merchants newest first, after the cursor in q.
func (s *Store) ListMerchants(
	ctx context.Context, q merchants.ListQuery,
) ([]merchants.Merchant, error) {
	if q.Limit < 1 || q.Limit > merchants.MaxPageSize+1 {
		return nil, merchants.ErrInvalidInput
	}
	p := sqlcgen.ListMerchantsParams{
		Status: string(q.Status), Search: q.Search, RowLimit: int32(q.Limit),
	}
	if q.After != nil {
		p.HasCursor = true
		p.AfterCreatedAt = ts(q.After.CreatedAt)
		p.AfterID = q.After.ID
	} else {
		p.AfterCreatedAt = pgtype.Timestamptz{}
	}
	rows, err := sqlcgen.New(s.pool).ListMerchants(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("list merchants: %w", err)
	}
	out := make([]merchants.Merchant, len(rows))
	for i, r := range rows {
		out[i] = merchantFromRow(sqlcgen.GetMerchantRow(r))
	}
	return out, nil
}

// ChangeStatus applies a guarded status change with its audit entries.
func (s *Store) ChangeStatus(
	ctx context.Context, u merchants.StatusUpdate, fx merchants.Effects,
) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		n, err := q.ChangeMerchantStatus(ctx, sqlcgen.ChangeMerchantStatusParams{
			ID: u.ID, FromStatus: string(u.From), ToStatus: string(u.To), Now: ts(u.At),
		})
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		_, err = q.GetMerchant(ctx, u.ID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return merchants.ErrNotFound
		case err != nil:
			return err
		}
		return merchants.ErrInvalidTransition
	})
}

// MerchantCounts counts merchants in each status.
func (s *Store) MerchantCounts(ctx context.Context) (map[merchants.Status]int, error) {
	rows, err := sqlcgen.New(s.pool).CountMerchantsByStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("count merchants: %w", err)
	}
	out := make(map[merchants.Status]int, len(rows))
	for _, r := range rows {
		out[merchants.Status(r.Status)] = int(r.N)
	}
	return out, nil
}

// CreatedPerDay counts merchants created on each Gabon day from since on.
func (s *Store) CreatedPerDay(
	ctx context.Context, since time.Time,
) ([]merchants.DayCount, error) {
	rows, err := sqlcgen.New(s.pool).CountMerchantsCreatedPerDay(ctx, ts(since))
	if err != nil {
		return nil, fmt.Errorf("count merchants per day: %w", err)
	}
	out := make([]merchants.DayCount, len(rows))
	for i, r := range rows {
		out[i] = merchants.DayCount{Day: r.Day.Time, Count: int(r.N)}
	}
	return out, nil
}
