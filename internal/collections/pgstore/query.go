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

// List returns a merchant's collections, newest first, one keyset page.
func (s *Store) List(
	ctx context.Context, q collections.ListQuery,
) ([]collections.Collection, error) {
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
	ctx context.Context, q collections.ListQuery, statuses []string, limit int32,
) ([]collections.Collection, error) {
	rows, err := sqlcgen.New(s.pool).ListCollectionsFirstPage(ctx,
		sqlcgen.ListCollectionsFirstPageParams{
			MerchantID: q.MerchantID, Statuses: statuses, Reference: q.Reference, BatchSize: limit,
		})
	return convertAll(rows, err, firstPageRow)
}

func (s *Store) pageAfter(
	ctx context.Context, q collections.ListQuery, statuses []string, limit int32,
) ([]collections.Collection, error) {
	rows, err := sqlcgen.New(s.pool).ListCollectionsAfter(ctx,
		sqlcgen.ListCollectionsAfterParams{
			AfterID: q.After, MerchantID: q.MerchantID, Statuses: statuses,
			Reference: q.Reference, BatchSize: limit,
		})
	return convertAll(rows, err, afterRow)
}

func convertAll[R any](
	rows []R, err error, conv func(R) collections.Collection,
) ([]collections.Collection, error) {
	if err != nil {
		return nil, err
	}
	out := make([]collections.Collection, 0, len(rows))
	for _, r := range rows {
		out = append(out, conv(r))
	}
	return out, nil
}

// The list queries select the same columns as GetCollection, so their row
// types convert directly.
func firstPageRow(r sqlcgen.ListCollectionsFirstPageRow) collections.Collection {
	return fromRow(sqlcgen.GetCollectionRow(r))
}

func afterRow(r sqlcgen.ListCollectionsAfterRow) collections.Collection {
	return fromRow(sqlcgen.GetCollectionRow(r))
}
