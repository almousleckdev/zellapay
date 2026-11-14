// Package pgstore persists the ledger in PostgreSQL.
package pgstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/pgstore/sqlcgen"
)

// Store implements ledger.Poster and ledger.Reader on PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

var (
	_ ledger.Poster = (*Store)(nil)
	_ ledger.Reader = (*Store)(nil)
)

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Post writes j atomically. A repeat of an identical journal under the same
// idempotency key writes nothing and reports Replayed.
func (s *Store) Post(ctx context.Context, j ledger.Journal) (ledger.PostResult, error) {
	if err := j.Validate(); err != nil {
		return ledger.PostResult{}, err
	}
	var res ledger.PostResult
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		res, err = post(ctx, sqlcgen.New(tx), j)
		return err
	})
	if err != nil {
		return ledger.PostResult{}, fmt.Errorf("post journal %s: %w", j.ID, err)
	}
	return res, nil
}

func post(ctx context.Context, q *sqlcgen.Queries, j ledger.Journal) (ledger.PostResult, error) {
	accounts, err := lockAccounts(ctx, q, j)
	if err != nil {
		return ledger.PostResult{}, err
	}
	fp := j.Fingerprint()
	_, err = q.InsertJournal(ctx, sqlcgen.InsertJournalParams{
		ID:             j.ID,
		IdempotencyKey: j.IdempotencyKey,
		Kind:           j.Kind,
		Reference:      j.Reference,
		Fingerprint:    fp[:],
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return replay(ctx, q, j, fp[:])
	}
	if err != nil {
		return ledger.PostResult{}, err
	}
	if err := insertEntries(ctx, q, j); err != nil {
		return ledger.PostResult{}, err
	}
	if err := checkOverdraft(ctx, q, accounts); err != nil {
		return ledger.PostResult{}, err
	}
	return ledger.PostResult{JournalID: j.ID}, nil
}
