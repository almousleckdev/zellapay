package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/payouts/pgstore/sqlcgen"
)

// Update locks the payout, applies fn and stores the result and its
// event in one transaction.
func (s *Store) Update(
	ctx context.Context, id string, fn payouts.UpdateFunc,
) (payouts.Payout, error) {
	var result payouts.Payout
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		row, err := q.LockPayout(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return payouts.ErrNotFound
		}
		if err != nil {
			return err
		}
		current := fromRow(sqlcgen.GetPayoutRow(row))
		next, ev, err := fn(current)
		if errors.Is(err, payouts.ErrNoChange) {
			result = current
			return nil
		}
		if err != nil {
			return err
		}
		result = next
		return persist(ctx, q, next, ev)
	})
	return result, err
}

func persist(
	ctx context.Context, q *sqlcgen.Queries, c payouts.Payout, ev payouts.Event,
) error {
	if err := q.UpdatePayout(ctx, updateParams(c)); err != nil {
		return err
	}
	if ev.Type == "" {
		return nil
	}
	if err := q.InsertPayoutEvent(ctx, eventParams(c.ID, ev)); err != nil {
		return err
	}
	return announceFinal(ctx, q, c, ev)
}

// announceFinal writes the outbox row in the same transaction as the status
// change, so the event exists exactly when the final status does.
func announceFinal(
	ctx context.Context, q *sqlcgen.Queries, c payouts.Payout, ev payouts.Event,
) error {
	typ, final := payouts.OutboxType(c.Status)
	if !final || ev.From == ev.To {
		return nil
	}
	return q.InsertPayoutOutbox(ctx, sqlcgen.InsertPayoutOutboxParams{
		PayoutID: c.ID, EventType: typ, CreatedAt: ts(ev.At),
	})
}

// Record appends an event without changing the payout.
func (s *Store) Record(ctx context.Context, id string, ev payouts.Event) error {
	return sqlcgen.New(s.pool).InsertPayoutEvent(ctx, eventParams(id, ev))
}
