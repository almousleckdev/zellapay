package pgstore

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/collections/pgstore/sqlcgen"
)

const maxBatch = 1000

// Due returns open collections whose next check time has passed.
func (s *Store) Due(
	ctx context.Context, now time.Time, limit int,
) ([]collections.Collection, error) {
	if limit < 1 || limit > maxBatch {
		limit = maxBatch
	}
	rows, err := sqlcgen.New(s.pool).ListDueCollections(ctx, sqlcgen.ListDueCollectionsParams{
		Now: ts(now), BatchSize: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]collections.Collection, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(sqlcgen.GetCollectionRow(r)))
	}
	return out, nil
}

// Events returns a collection's audit trail, oldest first.
func (s *Store) Events(ctx context.Context, id string) ([]collections.Event, error) {
	rows, err := sqlcgen.New(s.pool).ListCollectionEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]collections.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, collections.Event{
			Type: r.Type, From: collections.Status(r.FromStatus),
			To: collections.Status(r.ToStatus), Detail: r.Detail, At: timeOf(r.CreatedAt),
		})
	}
	return out, nil
}

func timeOf(t pgtype.Timestamptz) time.Time { return t.Time }
