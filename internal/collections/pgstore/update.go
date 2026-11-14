package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/collections/pgstore/sqlcgen"
)

// Update locks the collection, applies fn and stores the result and its
// event in one transaction.
func (s *Store) Update(
	ctx context.Context, id string, fn collections.UpdateFunc,
) (collections.Collection, error) {
	var result collections.Collection
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		row, err := q.LockCollection(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return collections.ErrNotFound
		}
		if err != nil {
			return err
		}
		current := fromRow(sqlcgen.GetCollectionRow(row))
		next, ev, err := fn(current)
		if errors.Is(err, collections.ErrNoChange) {
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
	ctx context.Context, q *sqlcgen.Queries, c collections.Collection, ev collections.Event,
) error {
	if err := q.UpdateCollection(ctx, updateParams(c)); err != nil {
		return err
	}
	if ev.Type == "" {
		return nil
	}
	return q.InsertCollectionEvent(ctx, eventParams(c.ID, ev))
}

// Record appends an event without changing the collection.
func (s *Store) Record(ctx context.Context, id string, ev collections.Event) error {
	return sqlcgen.New(s.pool).InsertCollectionEvent(ctx, eventParams(id, ev))
}
