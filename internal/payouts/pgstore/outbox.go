package pgstore

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/payouts/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

// ClaimOutbox offers unpublished outbox entries to handle inside one
// transaction. Rows locked by another caller are skipped, and entries are
// marked published only when handle returns nil.
func (s *Store) ClaimOutbox(
	ctx context.Context, now time.Time, limit int, handle payouts.OutboxHandler,
) (int, error) {
	if limit < 1 || limit > maxBatch {
		limit = maxBatch
	}
	claimed := 0
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		rows, err := q.ClaimPayoutOutbox(ctx, pgconv.Int32(limit))
		if err != nil || len(rows) == 0 {
			return err
		}
		entries := make([]payouts.OutboxEntry, len(rows))
		ids := make([]int64, len(rows))
		for i, r := range rows {
			entries[i] = payouts.OutboxEntry{
				ID: r.ID, PayoutID: r.PayoutID,
				EventType: r.EventType, CreatedAt: r.CreatedAt.Time,
			}
			ids[i] = r.ID
		}
		if err := handle(ctx, entries); err != nil {
			return err
		}
		claimed = len(rows)
		return q.MarkPayoutOutboxPublished(ctx, sqlcgen.MarkPayoutOutboxPublishedParams{
			PublishedAt: ts(now), Ids: ids,
		})
	})
	return claimed, err
}
