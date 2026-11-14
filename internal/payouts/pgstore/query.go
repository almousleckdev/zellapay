package pgstore

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/payouts/pgstore/sqlcgen"
)

const maxBatch = 1000

// Due returns open payouts whose next check time has passed.
func (s *Store) Due(
	ctx context.Context, now time.Time, limit int,
) ([]payouts.Payout, error) {
	if limit < 1 || limit > maxBatch {
		limit = maxBatch
	}
	rows, err := sqlcgen.New(s.pool).ListDuePayouts(ctx, sqlcgen.ListDuePayoutsParams{
		Now: ts(now), BatchSize: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]payouts.Payout, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(sqlcgen.GetPayoutRow(r)))
	}
	return out, nil
}

// Events returns a payout's audit trail, oldest first.
func (s *Store) Events(ctx context.Context, id string) ([]payouts.Event, error) {
	rows, err := sqlcgen.New(s.pool).ListPayoutEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]payouts.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, payouts.Event{
			Type: r.Type, From: payouts.Status(r.FromStatus),
			To: payouts.Status(r.ToStatus), Detail: r.Detail, At: timeOf(r.CreatedAt),
		})
	}
	return out, nil
}

func timeOf(t pgtype.Timestamptz) time.Time { return t.Time }

// List returns a merchant's payouts, newest first, one keyset page.
func (s *Store) List(
	ctx context.Context, q payouts.ListQuery,
) ([]payouts.Payout, error) {
	limit := q.Limit
	if limit < 1 || limit > maxBatch {
		limit = maxBatch
	}
	statuses := make([]string, len(q.Statuses))
	for i, st := range q.Statuses {
		statuses[i] = string(st)
	}
	if q.After == "" {
		return s.firstPage(ctx, q, statuses, int32(limit))
	}
	return s.pageAfter(ctx, q, statuses, int32(limit))
}

func (s *Store) firstPage(
	ctx context.Context, q payouts.ListQuery, statuses []string, limit int32,
) ([]payouts.Payout, error) {
	rows, err := sqlcgen.New(s.pool).ListPayoutsFirstPage(ctx,
		sqlcgen.ListPayoutsFirstPageParams{
			MerchantID: q.MerchantID, Statuses: statuses, Reference: q.Reference, BatchSize: limit,
		})
	return convertAll(rows, err, firstPageRow)
}

func (s *Store) pageAfter(
	ctx context.Context, q payouts.ListQuery, statuses []string, limit int32,
) ([]payouts.Payout, error) {
	rows, err := sqlcgen.New(s.pool).ListPayoutsAfter(ctx,
		sqlcgen.ListPayoutsAfterParams{
			AfterID: q.After, MerchantID: q.MerchantID, Statuses: statuses,
			Reference: q.Reference, BatchSize: limit,
		})
	return convertAll(rows, err, afterRow)
}

func convertAll[R any](
	rows []R, err error, conv func(R) payouts.Payout,
) ([]payouts.Payout, error) {
	if err != nil {
		return nil, err
	}
	out := make([]payouts.Payout, 0, len(rows))
	for _, r := range rows {
		out = append(out, conv(r))
	}
	return out, nil
}

// The list queries select the same columns as GetPayout, so their row
// types convert directly.
func firstPageRow(r sqlcgen.ListPayoutsFirstPageRow) payouts.Payout {
	return fromRow(sqlcgen.GetPayoutRow(r))
}

func afterRow(r sqlcgen.ListPayoutsAfterRow) payouts.Payout {
	return fromRow(sqlcgen.GetPayoutRow(r))
}
