// Package pgstore keeps idempotency records in PostgreSQL.
package pgstore

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/platform/idempotency"
	"github.com/zellapay/zellapay/internal/platform/idempotency/pgstore/sqlcgen"
)

// Options sets record lifetimes.
type Options struct {
	// TTL is how long a completed record is kept for replay.
	TTL time.Duration
	// LockTTL is how long an in-progress claim is honoured. After that a
	// retry with the same request may take it over.
	LockTTL time.Duration
}

// Store implements idempotency.Store on PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
	opts Options
}

var _ idempotency.Store = (*Store)(nil)

// New returns a Store. Zero options default to 24h and 60s.
func New(pool *pgxpool.Pool, opts Options) *Store {
	if opts.TTL <= 0 {
		opts.TTL = 24 * time.Hour
	}
	if opts.LockTTL <= 0 {
		opts.LockTTL = time.Minute
	}
	return &Store{pool: pool, opts: opts}
}

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// Begin claims the key, takes over a stale claim, or reports why not.
func (s *Store) Begin(
	ctx context.Context, req idempotency.Request, now time.Time,
) (idempotency.Decision, error) {
	q := sqlcgen.New(s.pool)
	for attempt := 0; attempt < 3; attempt++ {
		started, err := s.claim(ctx, q, req, now)
		if err != nil {
			return idempotency.Decision{}, err
		}
		if started {
			return idempotency.Decision{Kind: idempotency.Started}, nil
		}
		d, found, err := s.inspect(ctx, q, req)
		if err != nil || found {
			return d, err
		}
	}
	return idempotency.Decision{Kind: idempotency.InFlight}, nil
}

// claim tries to insert a new record, then to take over a stale one.
func (s *Store) claim(
	ctx context.Context, q *sqlcgen.Queries, req idempotency.Request, now time.Time,
) (bool, error) {
	locked, expires := ts(now.Add(s.opts.LockTTL)), ts(now.Add(s.opts.TTL))
	_, err := q.InsertKey(ctx, sqlcgen.InsertKeyParams{
		Scope: req.Scope, Key: req.Key, Fingerprint: req.Fingerprint,
		LockedUntil: locked, Now: ts(now), ExpiresAt: expires,
	})
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	_, err = q.TakeOverKey(ctx, sqlcgen.TakeOverKeyParams{
		Scope: req.Scope, Key: req.Key, Fingerprint: req.Fingerprint,
		LockedUntil: locked, Now: ts(now), ExpiresAt: expires,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// inspect reads the existing record. found is false if it vanished
// between the claim attempt and the read, in which case the caller retries.
func (s *Store) inspect(
	ctx context.Context, q *sqlcgen.Queries, req idempotency.Request,
) (idempotency.Decision, bool, error) {
	row, err := q.GetKey(ctx, sqlcgen.GetKeyParams{Scope: req.Scope, Key: req.Key})
	if errors.Is(err, pgx.ErrNoRows) {
		return idempotency.Decision{}, false, nil
	}
	if err != nil {
		return idempotency.Decision{}, false, err
	}
	switch {
	case !bytes.Equal(row.Fingerprint, req.Fingerprint):
		return idempotency.Decision{Kind: idempotency.Conflict}, true, nil
	case row.State == "complete":
		return idempotency.Decision{
			Kind: idempotency.Replay,
			Response: idempotency.Response{
				Status:      int(row.ResponseStatus.Int32),
				ContentType: row.ResponseContentType.String,
				Body:        row.ResponseBody,
			},
		}, true, nil
	}
	return idempotency.Decision{Kind: idempotency.InFlight}, true, nil
}
