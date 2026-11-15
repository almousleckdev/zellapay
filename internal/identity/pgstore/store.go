// Package pgstore persists identity data in PostgreSQL.
package pgstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

const uniqueViolation = "23505"

var ts = pgconv.Timestamptz

// MailQueue stores an email in the notification outbox inside a transaction.
type MailQueue interface {
	InsertIn(ctx context.Context, tx pgx.Tx, p notify.Prepared) (bool, error)
}

// AuditLog appends an entry to the audit log inside a transaction.
type AuditLog interface {
	AppendIn(ctx context.Context, tx pgx.Tx, p audit.Pending) (audit.Record, error)
}

// Store implements identity.Repository on PostgreSQL.
type Store struct {
	pool  *pgxpool.Pool
	mail  MailQueue
	audit AuditLog
}

var _ identity.Repository = (*Store)(nil)

// New returns a Store. mail and audit are other modules' stores; they write
// to their own tables, in the transaction this Store opens.
func New(pool *pgxpool.Pool, mail MailQueue, auditLog AuditLog) *Store {
	return &Store{pool: pool, mail: mail, audit: auditLog}
}

// inTx runs work, then queues the effects, in one transaction.
func (s *Store) inTx(
	ctx context.Context, fx identity.Effects, work func(q *sqlcgen.Queries) error,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := work(sqlcgen.New(tx)); err != nil {
			return err
		}
		return s.queue(ctx, tx, fx)
	})
}

// queue stores the emails and audit entries inside tx. The audit entries come
// last because they lock the shared audit head.
func (s *Store) queue(ctx context.Context, tx pgx.Tx, fx identity.Effects) error {
	for _, m := range fx.Mail {
		if _, err := s.mail.InsertIn(ctx, tx, m); err != nil {
			return fmt.Errorf("queue email: %w", err)
		}
	}
	for _, e := range fx.Audit {
		if _, err := s.audit.AppendIn(ctx, tx, e); err != nil {
			return fmt.Errorf("append audit entry: %w", err)
		}
	}
	return nil
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == uniqueViolation
}
