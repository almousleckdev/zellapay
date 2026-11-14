package pgstore

import (
	"context"
	"time"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/collections/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

// Unsettled returns succeeded collections completed before the cutoff whose
// funds have not been released. Only the money fields are filled in.
func (s *Store) Unsettled(
	ctx context.Context, before time.Time, limit int,
) ([]collections.Collection, error) {
	if limit < 1 || limit > maxBatch {
		limit = maxBatch
	}
	rows, err := sqlcgen.New(s.pool).ListUnsettledCollections(ctx,
		sqlcgen.ListUnsettledCollectionsParams{Before: ts(before), BatchSize: pgconv.Int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]collections.Collection, len(rows))
	for i, r := range rows {
		out[i] = collections.Collection{
			ID: r.ID, MerchantID: r.MerchantID, Amount: r.Amount, Currency: r.Currency,
			Fee: r.FeeAmount, Status: collections.StatusSucceeded,
			CompletedAt: timePtr(r.CompletedAt),
		}
	}
	return out, nil
}

// MarkSettled records that the collection's funds were released.
func (s *Store) MarkSettled(ctx context.Context, id string, at time.Time) error {
	return sqlcgen.New(s.pool).MarkCollectionSettled(ctx,
		sqlcgen.MarkCollectionSettledParams{ID: id, SettledAt: ts(at)})
}
