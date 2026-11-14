// Package pgstore keeps the notification outbox in PostgreSQL.
package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/notify/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

const maxBatch = 200

var (
	ts      = pgconv.Timestamptz
	timePtr = pgconv.TimePtr
)

// Store implements notify.Repository on PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

var _ notify.Repository = (*Store)(nil)

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Insert stores p, reporting false if its dedupe key already exists.
func (s *Store) Insert(ctx context.Context, p notify.Prepared) (bool, error) {
	return insert(ctx, sqlcgen.New(s.pool), p)
}

// InsertIn stores p inside tx. Use it to queue a message in the same
// transaction as the change that caused it: if the transaction rolls back,
// the message never existed.
func (s *Store) InsertIn(ctx context.Context, tx pgx.Tx, p notify.Prepared) (bool, error) {
	return insert(ctx, sqlcgen.New(tx), p)
}

func insert(ctx context.Context, q *sqlcgen.Queries, p notify.Prepared) (bool, error) {
	n, err := q.InsertNotification(ctx, sqlcgen.InsertNotificationParams{
		ID: p.ID, Channel: string(p.Channel), Template: p.Template, Locale: p.Locale,
		RecipientMasked: p.RecipientMasked, PayloadEnc: p.PayloadSealed,
		DedupeKey: pgtype.Text{String: p.DedupeKey, Valid: p.DedupeKey != ""},
		CreatedAt: ts(p.CreatedAt), RetryUntil: ts(p.RetryUntil),
	})
	return n == 1, err
}

// ClaimDue returns due messages and pushes their next attempt forward by
// lease, so a second worker cannot send the same message while the first is
// sending. If a worker dies, the lease expires and the message is retried.
func (s *Store) ClaimDue(
	ctx context.Context, now time.Time, limit int, lease time.Duration,
) ([]notify.Queued, error) {
	if limit < 1 || limit > maxBatch {
		limit = maxBatch
	}
	rows, err := sqlcgen.New(s.pool).ClaimDueNotifications(ctx, sqlcgen.ClaimDueNotificationsParams{
		Now: ts(now), BatchSize: pgconv.Int32(limit), LeaseUntil: ts(now.Add(lease)),
	})
	if err != nil {
		return nil, err
	}
	out := make([]notify.Queued, len(rows))
	for i, r := range rows {
		out[i] = notify.Queued{
			ID: r.ID, Channel: notify.Channel(r.Channel), Template: r.Template, Locale: r.Locale,
			PayloadSealed: r.PayloadEnc, Attempts: int(r.Attempts), RetryUntil: r.RetryUntil.Time,
		}
	}
	return out, nil
}

// Record stores an attempt's outcome. A sent or dead message has its payload
// erased by the same statement.
func (s *Store) Record(ctx context.Context, id string, r notify.Result) error {
	next := pgtype.Timestamptz{}
	if r.NextAttemptAt != nil {
		next = ts(*r.NextAttemptAt)
	}
	sent := pgtype.Timestamptz{}
	if r.SentAt != nil {
		sent = ts(*r.SentAt)
	}
	return sqlcgen.New(s.pool).RecordNotification(ctx, sqlcgen.RecordNotificationParams{
		ID: id, Status: string(r.Status), Attempts: pgconv.Int32(r.Attempts),
		NextAttemptAt: next, LastError: r.LastError, SentAt: sent,
	})
}

// Get returns a message's state without its payload.
func (s *Store) Get(ctx context.Context, id string) (notify.Record, error) {
	r, err := sqlcgen.New(s.pool).GetNotification(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return notify.Record{}, notify.ErrNotFound
	}
	if err != nil {
		return notify.Record{}, err
	}
	return notify.Record{
		ID: r.ID, Channel: notify.Channel(r.Channel), Template: r.Template,
		RecipientMasked: r.RecipientMasked, Status: notify.Status(r.Status),
		Attempts: int(r.Attempts), LastError: r.LastError, CreatedAt: r.CreatedAt.Time,
		SentAt: timePtr(r.SentAt), PayloadErased: r.PayloadErased,
	}, nil
}
